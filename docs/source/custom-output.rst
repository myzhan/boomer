Custom output
=============

You can write you own output to deal with the test result.

.. code-block:: go

    type Output interface {
        OnStart()
        OnEvent(data map[string]interface{})
        OnStop()
    }

All the OnXXX function will be call in a separated goroutine, just in case some output will block.
But it will wait for all outputs return to avoid data lost.

It works like:

.. code-block:: go

    wg := sync.WaitGroup{}
    wg.Add(len(outputs))
    for _, output := range outputs {
        go func(o Output) {
            o.OnXXXX()
            wg.Done()
        }(output)
    }
    wg.Wait()

OnStart
-------
OnStart will be call before the test starts.

OnEvent
-------
By default, each output receive stats data from runner every three seconds.
OnEvent is responsible for dealing with the data.

Don't write to the origin data! Because all outputs share the same reference.

OnStop
------
OnStop will be called before the test ends. If you are writing to a disk file, it's time to flush.