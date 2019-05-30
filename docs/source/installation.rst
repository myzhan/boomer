Installation
============

Boomer can be installed and updated with the "go get" command.

install:

.. code-block:: console

    $ go get github.com/myzhan/boomer

update:

.. code-block:: console

    $ go get -u github.com/myzhan/boomer

If you want to point to a particular revision of boomer, you should use a dependency management
tool like `dep <https://github.com/golang/dep>`_ or go module.

The goczmq dependency
---------------------

Locust uses the zeromq protocol, so boomer depends on a zeromq client. Boomer uses
`gomq <https://github.com/zeromq/gomq>`_ by default, which is a pure Go implementation.

Because of the instability of gomq, you can switch to `goczmq <https://github.com/zeromq/goczmq>`_.

Once install goczmq successfully, then you can build with goczmq instead of gomq.

.. code-block:: console

    $ go build -tags 'goczmq' your-code.go

