=========
Profiling
=========

You may think there are bottlenecks in your load generator, don't hesitate to do profiling.

Both CPU and memory profiling are supported.

It's not suggested to run CPU profiling and memory profiling at the same time.


CPU Profiling
-------------

.. code-block:: console

    # 1. run locust master.
    # 2. run boomer with cpu profiling for 30 seconds.
    $ go run main.go -cpu-profile cpu.pprof -cpu-profile-duration 30s
    # 3. start test in the WebUI.
    # 4. run pprof.
    $ go tool pprof cpu.pprof
    Type: cpu
    Time: Nov 14, 2018 at 8:04pm (CST)
    Duration: 30.17s, Total samples = 12.07s (40.01%)
    Entering interactive mode (type "help" for commands, "o" for options)
    (pprof) web


Memory Profiling
----------------

.. code-block:: console

    # 1. run locust master.
    # 2. run boomer with memory profiling for 30 seconds.
    $ go run main.go -mem-profile mem.pprof -mem-profile-duration 30s
    # 3. start test in the WebUI.
    # 4. run pprof and try 'go tool pprof --help' to learn more.
    $ go tool pprof -alloc_space mem.pprof
    Type: alloc_space
    Time: Nov 14, 2018 at 8:26pm (CST)
    Entering interactive mode (type "help" for commands, "o" for options)
    (pprof) top
