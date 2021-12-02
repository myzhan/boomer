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

You can write you own output and add more by calling boomer.AddOutput().

Here is an example for writting to stdout.

.. code-block:: go

   AddOutput(new boomer.NewConsoleOutput())

Here is an example for pushing output to Prometheus Pushgateway.

.. code-block:: go

   var globalBoomer *boomer.Boomer

   func main() {
      task1 := &boomer.Task{
         Name:   "foo",
         Weight: 10,
         Fn:     foo,
      }

      numClients := 10
      spawnRate := float64(10)
      globalBoomer = boomer.NewStandaloneBoomer(numClients, spawnRate)
      globalBoomer.AddOutput(boomer.NewPrometheusPusherOutput("http://localhost:9091", "hrp"))
      globalBoomer.Run(task1)
   }


.. literalinclude:: ../../examples/standalone/standalone.go
   :language: go
   :linenos:
   :emphasize-lines: 33
