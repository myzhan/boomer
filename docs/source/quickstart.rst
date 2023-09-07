Quickstart
==========


Code
----

This is a quick example of writing test scenarios with boomer.

.. code-block:: go

    package main

    import "time"
    import "github.com/myzhan/boomer"

    func foo(){
        start := time.Now()
        time.Sleep(100 * time.Millisecond)
        elapsed := time.Since(start)

        /*
        Report your test result as a success
        */
        boomer.RecordSuccess("http", "foo", elapsed.Nanoseconds()/int64(time.Millisecond), int64(10))
    }

    func bar(){
        start := time.Now()
        time.Sleep(100 * time.Millisecond)
        elapsed := time.Since(start)

        /*
        Report your test result as a failure
        */
        boomer.RecordFailure("udp", "bar", elapsed.Nanoseconds()/int64(time.Millisecond), "udp error")
    }

    func main(){
        task1 := &boomer.Task{
            Name: "foo",
            // The weight is used to distribute goroutines over multiple tasks.
            Weight: 10,
            Fn: foo,
        }

        task2 := &boomer.Task{
            Name: "bar",
            Weight: 20,
            Fn: bar,
        }

        boomer.Run(task1, task2)
    }


Here we define two tasks, task1 reports a success to boomer every 100 milliseconds, and meanwhile
task2 reports a failure. The weight of task1 is 10, and the weight of task2 is 20, if the locust
master asks boomer to spawn 30 users, then task1 will get 10 goroutines to run and task2 will get 20.
The numbers of users can be specified in the Web UI.


Test
-----

Because task1 is named foo and tasks2 is named bar, you can run them without connecting to the master.

.. code-block:: console

    $ go run your-code.go --run-tasks foo,bar

In this case, task1 and task2 will be run for one time with no output.

You can add logs to ensure your tasks running correctly.


Build
-----

.. code-block:: console

    $ go build -o your-code your-code.go


Run
---

1. Start the locust master with the included dummy.py.

.. code-block:: console

    $ locust --master -f dummy.py

So far, dummy.py is necessary when starting a master, because locust needs such a file.

Don't worry, dummy.py has nothing to do with your test.

2. Start your test program.

.. code-block:: console

    $ chmod +x ./your-code && ./your-code

.. note::

    To see all available options type: ``your-code --help``


Open up Locust's web interface
------------------------------

Once you've started Locust and boomer, you should open up a browser and point it to http://127.0.0.1:8089 (if you are running Locust locally).