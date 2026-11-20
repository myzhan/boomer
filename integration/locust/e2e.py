"""Run a real Locust master against a separately built Boomer worker."""

# Import Locust first so gevent patches subprocess and networking before use.
from locust import User, task
from locust.env import Environment
from locust.event import Events

import json
import logging
import os
from pathlib import Path
import signal
import subprocess
import sys
import time

import gevent
from gevent.pywsgi import WSGIServer


class BoomerUser(User):
    @task
    def placeholder(self):
        raise AssertionError("the master must delegate execution to Boomer")


def wait_until(description, predicate, worker, timeout=20):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if predicate():
            return
        if worker.poll() is not None:
            raise AssertionError(f"worker exited early ({worker.returncode}): {description}")
        gevent.sleep(0.1)
    raise AssertionError(f"timed out: {description}")


def main():
    binary = str(Path(sys.argv[1]).resolve())
    artifacts = Path(sys.argv[2])
    artifacts.mkdir(parents=True, exist_ok=True)
    logging.basicConfig(
        level=logging.INFO,
        handlers=[logging.FileHandler(artifacts / "master.log"), logging.StreamHandler()],
    )
    requests = {"/success": 0, "/failure": 0}

    def target(environ, start_response):
        path = environ["PATH_INFO"]
        requests[path] += 1
        gevent.sleep(0.01)
        status = "200 OK" if path == "/success" else "503 Service Unavailable"
        start_response(status, [("Content-Type", "text/plain")])
        return [b"boomer-e2e"]

    server = WSGIServer(("127.0.0.1", 0), target, log=None)
    server.start()
    environment = Environment(user_classes=[BoomerUser], events=Events())
    master = environment.create_master_runner(master_bind_host="127.0.0.1", master_bind_port=0)
    reports = []

    @environment.events.worker_report.add_listener
    def on_report(client_id, data, **kwargs):
        reports.append({"client_id": client_id, "data": data})

    worker = None
    snapshots = []
    try:
        with (artifacts / "worker.log").open("w") as worker_log:
            worker = subprocess.Popen(
                [binary, "--master-host=127.0.0.1", f"--master-port={master.server.port}"],
                env={**os.environ, "BOOMER_E2E_TARGET": f"http://127.0.0.1:{server.server_port}"},
                stdout=worker_log,
                stderr=subprocess.STDOUT,
            )
            wait_until("worker registration", lambda: len(master.clients.ready) == 1, worker)
            for cycle, users in enumerate((4, 2), start=1):
                previous_reports = len(reports)
                previous_requests = dict(requests)
                master.start(user_count=users, spawn_rate=users)
                wait_until(
                    "spawn completion",
                    lambda: master.spawning_completed and master.user_count == users,
                    worker,
                )

                def stats_received():
                    success = master.stats.entries.get(("/success", "GET"))
                    failure = master.stats.entries.get(("/failure", "GET"))
                    return (
                        len(reports) >= previous_reports + 2
                        and success is not None and success.num_requests >= 10
                        and failure is not None and failure.num_failures >= 10
                    )

                wait_until("two stats reports with success and failure samples", stats_received, worker)
                success = master.stats.entries[("/success", "GET")]
                failure = master.stats.entries[("/failure", "GET")]
                total = master.stats.total
                assert success.num_failures == 0
                assert success.total_content_length == success.num_requests * len(b"boomer-e2e")
                assert success.min_response_time > 0 and success.max_response_time >= success.min_response_time
                assert sum(success.response_times.values()) == success.num_requests
                assert failure.num_requests == failure.num_failures
                assert total.num_requests == success.num_requests + failure.num_requests
                assert total.num_failures == failure.num_failures
                assert any(error.error == "HTTP 503" and error.occurrences > 0 for error in master.stats.errors.values())
                assert reports[-1]["data"]["user_count"] == users
                assert sum(reports[-1]["data"]["user_classes_count"].values()) == users
                assert all(requests[path] > previous_requests[path] for path in requests)
                snapshots.append({"cycle": cycle, "users": users, "stats": master.stats.serialize_stats(), "total": total.serialize()})
                master.stop()
                wait_until(
                    "worker stopped and ready for the next test",
                    lambda: master.user_count == 0 and len(master.clients.ready) == 1,
                    worker,
                )

            # Exercise Boomer's public process shutdown path and its quit notification.
            worker.send_signal(signal.SIGTERM)
            assert worker.wait(timeout=15) == 0, "worker failed during graceful shutdown"
            deadline = time.monotonic() + 5
            while len(master.clients.all) and time.monotonic() < deadline:
                gevent.sleep(0.1)
            assert not master.clients.all, "master did not receive the worker quit notification"
        logs = (artifacts / "worker.log").read_text()
        assert "DATA RACE" not in logs and "panic:" not in logs, logs
        assert "Traceback" not in (artifacts / "master.log").read_text()
        print("PASS: registration, spawn, real HTTP requests, stats, stop, restart, graceful quit")
    finally:
        (artifacts / "reports.json").write_text(json.dumps(reports, indent=2))
        (artifacts / "stats.json").write_text(json.dumps(snapshots, indent=2))
        if worker is not None and worker.poll() is None:
            worker.kill()
            worker.wait(timeout=5)
        master.quit()
        server.stop()


if __name__ == "__main__":
    main()
