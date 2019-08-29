Getting Started
=================

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

1. Download the release packages from https://github.com/fractal-platform/fractal/releases.
2. Start the terminal application.
3. Decompress the release packages to the distinct directory. Run these commands in terminal: 

.. code-block:: bash

    mkdir ~/fractal-test
    cd ~/fractal-test
    mv <download path>/fractal-0.2.0.tgz .
    tar zxvf fractal-0.2.0.tgz

4. Setup enviroment. Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    . setenv.sh

5. Test installation. Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    gftl -h

If you get command help in terminal, it means that your installation is OK.

Create your account
------------------------------------------
You can create your account in two ways:

* Use gtool command line. Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    . setenv.sh
    gtool keys --keys data/keys --pass [mypassword] newkeys

Then you can get your account address in terminal output.

.. hint::   You should set your own [mypassword] here, it is set to protect your private keys. Visit `here <../refs/gtool.html>`_ for more information about gtool command line tool.

* Use Fractal Wallet Application

*Please reference the documents of fractal-wallet*

How to Get Stake on Fractal Testnet
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
You can get stake in two ways:

* Request stake in the website: http://stake.fractalblock.com.
* Ask your friend to transfer stake to you.

How to Check Your Stake on Fractal Testnet
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
You can get stake in two ways:

* Use gtool command line. Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    . setenv.sh
    gtool state --rpc [rpc address] --addr [account address] account

Then you can get your account balance in terminal output.

.. hint::   You should set [rpc address] and [account address] here. [rpc address] is http://127.0.0.1:8545 for local node. [account address] is the account address produced when you create your account. Visit `here <../refs/gtool.html>`_ for more information about gtool command line tool.

* Find account details in the website: http://testnet.fractalblock.com.

Deploy node without mining
------------------------------------------
Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    . setenv.sh
    gftl --testnet3 --rpc --datadir data --unlock [mypassword]

.. hint::   [mypassword] is the password when you create your account. Visit `here <../refs/gtool.html>`_ for more information about gtool command line tool.

Start another terminal to check status. Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    . setenv.sh
    gtool block --rpc [rpc address] --height 100 query

Then you can get the block detail with 100-height in terminal output.

.. hint::   [rpc address] is http://127.0.0.1:8545 for local node. Visit `here <../refs/gtool.html>`_ for more information about gtool command line tool.

Deploy miner node
------------------------------------------
1. First, you must check your account stake balance. Since Fractal is proof-of-stake, you must hold some stakes to start mining.
2. Register mining keys. Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    . setenv.sh
    gtool keys --rpc [rpc address] --keys data/keys --pass [mypassword] --chainid [chainid] regminingkey

.. hint::   [rpc address] is http://127.0.0.1:8545 for local node. [mypassword] is the password when you create your account. [chainid] is 4 for testnet3. Visit `here <../refs/gtool.html>`_ for more information about gtool command line tool.

3. Start miner node. Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    . setenv.sh
    gftl --rpc --testnet3 --datadir data --unlock [mypassword] --mine

.. hint::   [mypassword] is the password when you create your account. Visit `here <../refs/gtool.html>`_ for more information about gtool command line tool.

Send transaction
------------------------------------------
Transfer Token
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    . setenv.sh
    gtool tx --rpc [rpc address] --keys data/keys --password [mypassword] --to [account address] --value [number] --chainid [chainid] send

.. hint::   [rpc address] is http://127.0.0.1:8545 for local node. [mypassword] is the password when you create your account. [account address] is a valid account address. [number] is the token amount you want to transfer. [chainid] is 4 for testnet3. Visit `here <../refs/gtool.html>`_ for more information about gtool command line tool.


