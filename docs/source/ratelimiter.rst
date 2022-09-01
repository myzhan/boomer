Ratelimiter
============

Ratelimiter is a feature like locust's LoadTestShape, used to control the executions of tasks.

Interface
---------

Boomer defines a interface named ratelimiter, you can write your own ratelimiter.

.. code-block:: go

    type RateLimiter interface {
        // Start is used to enable the rate limiter.
        // It can be implemented as a noop if not needed.
        Start()

        // Acquire() is called before executing a task.Fn function.
        // If Acquire() returns true, the task.Fn function will be executed.
        // If Acquire() returns false, the task.Fn function won't be executed this time, but Acquire() will be called very soon.
        // It works like:
        // for {
        //      blocked := rateLimiter.Acquire()
        //      if !blocked {
        //	        task.Fn()
        //      }
        // }
        // Acquire() should block the caller until execution is allowed.
        Acquire() bool

        // Stop is used to disable the rate limiter.
        // It can be implemented as a noop if not needed.
        Stop()
    }


And boomer has some builtin implementations like StableRateLimiter and RampUpRateLimiter.

Here is an example for adding rateLimiter.

.. code-block:: go

   ratelimiter, _ := boomer.NewRampUpRateLimiter(1000, "100/1s", time.Second)
   SetRateLimiter(ratelimiter)