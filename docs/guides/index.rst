Getting Started
=================

.. note::   This page is written for v0.2.x

In this tutorial, you will learn how to:

- Install Fractal Applications on macOS and Linux(Centos,Ubuntu)
- Create your account
- Deploy node without mining on Fractal Testnet
- Deploy miner node on Fractal Testnet
- Send transactions on Fractal Testnet

.. image:: steps.png
    :width: 500px
    :align: center

Install Fractal Applications
------------------------------------------
Supported Operation Systems:

    * macOS(version: 10.14.6 or later)
    * CentOS Linux(version: 7.6.1810 or later)
    * Ubuntu Linux(version: 18.04.2 or later)
    * Amazon Linux 2

1. Start the terminal application.
2. Run install script.

.. code-block:: bash

    wget https://raw.githubusercontent.com/fractal-platform/fractal/v0.2.0/scripts/install.sh
    sudo bash install.sh

If you get VERSION in terminal, it means that your installation is OK.

Create your account
------------------------------------------
You can create your account in two ways:

* Use gtool command line. Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    gtool keys --keys data/keys --pass [mypassword] newkeys

Then you can get your account address in terminal output.

.. hint::   You should set your own [mypassword] here, it is set to protect your private keys. Visit `here <../refs/gtool.html>`_ for more information about gtool command line tool.

You can export your private key, so it can be imported to Wallet Application. Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    gtool keys --keys data/keys --pass [mypassword] export

* Use Fractal Wallet Application

Visit https://github.com/fractal-platform/pihsiu for more information about fractal wallet.

How to Get Stake on Fractal Testnet
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
You can get stake in two ways:

* Request stake in the website: http://token.fractalblock.com:8081.
* Ask your friend to transfer stake to you.

How to Check Your Stake on Fractal Testnet
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
You can check your stake in two ways:

* Use gtool command line. Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    gtool state --rpc [rpc address] --addr [account address] account

Then you can get your account balance in terminal output.

.. hint::   You should set [`rpc address <../refs/rpclist.html>`_] and [account address] here. [`rpc address <../refs/rpclist.html>`_] is http://127.0.0.1:8545 for local node. [account address] is the account address produced when you create your account. Visit `here <../refs/gtool.html>`_ for more information about gtool command line tool.

* Find account details in the website: http://testnet.fractalblock.com:8081.

Deploy node without mining
------------------------------------------
Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    gftl --testnet --rpc --datadir data --unlock [mypassword]

.. hint::   [mypassword] is the password when you create your account. Visit `here <../refs/gtool.html>`_ for more information about gtool command line tool.

Start another terminal to check status. Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    gtool block --rpc [rpc address] --height 0 query

Then you can get the genesis block detail in terminal output.

.. hint::   [`rpc address <../refs/rpclist.html>`_] is http://127.0.0.1:8545 for local node. Visit `here <../refs/gtool.html>`_ for more information about gtool command line tool.

Deploy miner node
------------------------------------------
1. First, you must check your account stake balance. Since Fractal is proof-of-stake, you must hold some stakes to start mining.
2. Register mining keys. Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    gtool keys --rpc [rpc address] --keys data/keys --pass [mypassword] --chainid [chainid] regminingkey

.. hint::   [`rpc address <../refs/rpclist.html>`_] is http://127.0.0.1:8545 for local node. [mypassword] is the password when you create your account. [chainid] is 2 for testnet. Visit `here <../refs/gtool.html>`_ for more information about gtool command line tool.

3. Start miner node. Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    gftl --rpc --testnet --datadir data --unlock [mypassword] --mine

.. hint::   [mypassword] is the password when you create your account. Visit `here <../refs/gtool.html>`_ for more information about gtool command line tool.

Send transaction
------------------------------------------
Transfer Token
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    gtool tx --rpc [rpc address] --keys data/keys --pass [mypassword] --to [account address] --value [number] --chainid [chainid] send

.. hint::   [`rpc address <../refs/rpclist.html>`_] is http://127.0.0.1:8545 for local node. [mypassword] is the password when you create your account. [account address] is a valid account address. [number] is the token amount you want to transfer. [chainid] is 2 for testnet. Visit `here <../refs/gtool.html>`_ for more information about gtool command line tool.


