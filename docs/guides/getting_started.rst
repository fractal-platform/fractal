Getting Started
=================

Let’s start your fractal journey! In this chapter, we’ll discuss:

- Installing fractal on macOS and Linux (Centos,Ubuntu)
- Getting your first block 
- More about ``start_private.sh``

Installation
--------------
Here we provide macOs and Linux (Centos,Ubuntu) versions. You can download from https://github.com/fractal-platform/fractal/releases

Deploy ``PrivateNetwork`` node
------------------------------------------

Start ``PrivateNetwork`` node
''''''''''''''''''''''''''''''''

Here we take macOS as example.

Step 1. decompress file to current directory 

.. code-block:: bash 

    $ tar -zxvf gftl.macos.v0.1.0.tar.gz  -C .   

Step 2. enter gftl.macos.v0.1.0

.. code-block:: bash 

    $ cd gftl.macos.v0.1.0   

Step 3. make it executable 

.. code-block:: bash 

    $ chmod +x start_private.sh 

Step 4. start node

.. code-block:: bash 

    $ ./start_private.sh 

Manage ``PrivateNetwork`` node
''''''''''''''''''''''''''''''''

1. start fractal node: use this command if you want to start fractal node; when you shut down your PC ,you can run this to get fractal node run again

.. code-block:: bash 

    $ ./start_private.sh

2. clean files: this command deletes all files to restore the original files state

.. code-block:: bash 

    $ ./start_private.sh del

3. check: this command checks whether the fractal node runs well

.. code-block:: bash 

    $ ./start_private.sh check

**WARNING** You may find ``curl`` is not installed: ``curl: command not found``, run ``sudo apt-get install curl`` on Ubuntu to install it ,or ``sudo yum install curl`` on Centos.

4. stop: it stops fractal node, shut it down

.. code-block:: bash 

    $ ./start_private.sh stop


Deploy ``TestNetwork`` node
------------------------------------------

Here we take macOS as example.

Start ``PrivateNetwork`` node
''''''''''''''''''''''''''''''''

Step 1. decompress file to current directory 

.. code-block:: bash 

    $ tar -zxvf gftl.macos.v0.1.0.tar.gz  -C .   

Step 2. enter gftl.macos.v0.1.0

.. code-block:: bash 

    $ cd gftl.macos.v0.1.0   

Step 3. make it executable 

.. code-block:: bash 

    $ chmod +x start.sh 
    
Step 4. start node

.. code-block:: bash 

    $ ./start_testnet.sh 

Manage ``TestNetwork`` node
''''''''''''''''''''''''''''''''

1. start fractal node: use this command if you want to start fractal node; when you shut down your PC ,you can run this to get fractal node run again

.. code-block:: bash 

    $ ./start_testnet.sh

2. clean files: this command deletes all files to restore the original files state

.. code-block:: bash 

    $ ./start_testnet.sh del

3. check: this command checks whether the fractal node runs well

.. code-block:: bash 

    $ ./start_testnet.sh check

**WARNING** You may find ``curl`` is not installed: ``curl: command not found``, run ``sudo apt-get install curl`` on Ubuntu to install it ,or ``sudo yum install curl`` on Centos.

4. stop: it stops fractal node, shut it down

.. code-block:: bash 

    $ ./start_testnet.sh stop


