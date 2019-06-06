Running Mode
============

Currently, boomer has two running mode, standalone and distributed.

Distributed
------------
When running in distributed mode, boomer will connect to a locust master and running
as a slave. It's the default running mode of boomer.

Standalone
----------
When running in standalone mode, boomer doesn't need to connect to a locust master
and start testing immediately.

By default, the standalone mode works with a ConsoleOutput, which will print the
test result to the console, you can write you own output and add more by calling
boomer.AddOutput().

.. literalinclude:: ../../examples/standalone/standalone.go
   :language: go
   :linenos:
   :emphasize-lines: 33
