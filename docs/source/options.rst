=======
Options
=======

For convenience, boomer supports several command line options.

Since it may conflict with user's code, I'm planning to remove this feature and allow users
to set options programmatically.

``--master-host``
-----------------
Host or IP address of locust master for distributed load testing.

Defaults to 127.0.0.1.

``--master-port``
-----------------
The port to connect to that is used by the locust master for distributed load testing.

Defaults to 5557.

``--run-tasks``
-----------------
Run tasks without connecting to the master, multiply tasks is separated by comma.

``--max-rps``
-----------------
Max RPS that boomer can generate, disabled by default.

--max-rps=100 means the max RPS is limit to 100.

Defaults to 0.

``--request-increase-rate``
----------------------------
Request increase rate, disabled by default.

--request-increase-rate=100/1s means the threshold will ramp up.

``--cpu-profile``
-------------------------
Enable CPU profiling and specify a file path to save the result.

``--cpu-profile-duration``
--------------------------
CPU profile duration.

The timer will start when the process starts.

Defaults to 30 seconds.

``--mem-profile``
-------------------------
Enable memory profiling and specify a file path to save the result.

``--mem-profile-duration``
---------------------------
Memory profile duration.

The timer will start when the process starts.

Defaults to 30 seconds.

