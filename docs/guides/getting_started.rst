Getting Started
=================

Let’s start your fractal journey! In this quick start, you will learn how to:

- Install fractal on macOS and Linux(Centos,Ubuntu)
- Deploy ``PrivateNetwork`` node
- Deploy ``TestNetwork`` node

Installation
--------------
Our node currently supports macOs and Linux (Centos,Ubuntu) versions. You can download them from
https://github.com/fractal-platform/fractal/releases

Deploy ``PrivateNetwork`` node
------------------------------------------

Start ``PrivateNetwork`` node
''''''''''''''''''''''''''''''''

 - Linux (Centos and Ubuntu)

Step 1.

Step 2.

Step 3.


 - macOS (Using the Terminal application)

Step 1. unzip downloaded release file

.. code-block:: bash

    $ tar -zxvf fractal-bin.macos.v0.1.0.tar  -C .

Step 2. cd to fractal-bin.macos.v0.1.0

.. code-block:: bash

    $ cd fractal-bin.macos.v0.1.0

Step 3. make it executable

.. code-block:: bash

    $ chmod +x start_private.sh

Step 4. start node

.. code-block:: bash

    $ ./start_private.sh

Manage ``PrivateNetwork`` node
''''''''''''''''''''''''''''''''

1. start fractal node: use this command to start fractal node; after reboot, you can run this to restart fractal node.

.. code-block:: bash

    $ ./start_private.sh

2. clean files: this command deletes all files to restore the original files state.

.. code-block:: bash

    $ ./start_private.sh del

3. check: this command checks the fractal node status.

.. code-block:: bash

    $ ./start_private.sh check

**WARNING** If you get the warning: ``curl: command not found``, run ``sudo apt-get install curl`` (Ubuntu) or ``sudo yum install curl`` (Centos) to install it.

4. stop: this command stops fractal node, shuts it down

.. code-block:: bash

    $ ./start_private.sh stop


Deploy ``TestNetwork`` node
------------------------------------------

 - Linux (Centos and Ubuntu)

Step 1.

Step 2.

Step 3.


 - macOS (Using the Terminal application)


Start ``TestNetwork`` node
''''''''''''''''''''''''''''''''

Step 1. unzip downloaded release file

.. code-block:: bash

    $ tar -zxvf fractal-bin.macos.v0.1.0.tar  -C .

Step 2. cd fractal-bin.macos.v0.1.0

.. code-block:: bash

    $ cd fractal-bin.macos.v0.1.0

Step 3. make it executable

.. code-block:: bash

    $ chmod +x start.sh

Step 4. start node

.. code-block:: bash

    $ ./start_testnet.sh

Manage ``TestNetwork`` node
''''''''''''''''''''''''''''''''

1. start fractal node: uuse this command to start fractal node; after reboot, you can run this to restart fractal node.

.. code-block:: bash

    $ ./start_testnet.sh

2. clean files: this command deletes all files to restore the original files state

.. code-block:: bash

    $ ./start_testnet.sh del

3. check: this command checks the fractal node status

.. code-block:: bash

    $ ./start_testnet.sh check

**WARNING** If you get the warning: ``curl: command not found``, run ``sudo apt-get install curl`` (Ubuntu) or ``sudo yum install curl`` (Centos) to install it.

4. stop: this command stops fractal node, shuts it down

.. code-block:: bash

    $ ./start_testnet.sh stop


