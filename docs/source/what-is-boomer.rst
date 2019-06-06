===============================
What is Boomer?
===============================

Boomer is a golang library and works with `Locust <http://locust.io>`_.

Using goroutines to run you code concurrently will outperform the gevent implementation in Locust.
That's why I created this project.

Remember, use it as a library, not a general-purpose benchmarking tool.

Features
========

* **Write user test scenarios in golang**

 Just put you test scenarios in a normal function, boomer will spawn goroutines to run the function
 for many times to produce stress.

* **Build-in rate limit support**

 You can put rate limit on each boomer instance, which is useful when you just want to evaluate if
 the target is able to handle specific requests per second, instead of exhausting the target.

* **Different output destination**

 You can write you own output implementation to collect the test result.