Getting Started
=================

In this tutorial, you will learn how to:

- Install Fractal Applications on macOS and Linux(Centos, Ubuntu)
- Create account
- Deploy node without mining on Fractal Testnet
- Deploy miner node on Fractal Testnet
- Initiate transactions on Fractal Testnet

.. image:: steps.png
    :width: 500px
    :align: center

Install Fractal Applications
------------------------------------------
Supported Operating Systems:

    * macOS(version: Mojave 10.14.6 or later)
    * CentOS Linux(version: 7.6.1810 or later)
    * Ubuntu Linux(version: 18.04.2 or later)
    * Amazon Linux 2

1. Download the release packages from https://github.com/fractal-platform/fractal/releases.
2. Open the terminal application.
3. Unzip the release packages. Run these commands in terminal:

.. code-block:: bash

    mkdir ~/fractal-test
    cd ~/fractal-test
    mv <download path>/fractal-0.2.0.tgz .
    tar zxvf fractal-0.2.0.tgz

4. Setup environment. Run these commands in terminal:

.. code-block:: bash

    cd ~/fractal-test
    . setenv.sh

5. Test installation. Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    gftl -h

If you see command help in terminal, it means the installation is successful.

Create your account
------------------------------------------
You can create account in two ways:

* Use gtool command line. Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    . setenv.sh
    gtool keys --keys data/keys --pass [mypassword] newkeys

Then you can get your account address from terminal output.

.. Note::   You should set your own [mypassword], which is set to protect your private keys. Visit `here <../refs/gtool.html>`_ for more information about gtool command line tool.

* Use Fractal Wallet Application

*Please refer to fractal-wallet documents*

How to Raise Stake on Fractal Testnet
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
You can raise stake in two ways:

* Request stake from the website: http://stake.fractalblock.com.
* Ask someone to transfer stake to you.

How to Check Your Stake on Fractal Testnet
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
You can check stake in two ways:

* Use gtool command line. Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    . setenv.sh
    gtool state --rpc [rpc address] --addr [account address] account

Then you can get account balance from terminal output.

.. Note::   You should set your own [rpc address] and [account address]. Local node [rpc address] is http://127.0.0.1:8545. [account address] is the account address produced when you create your account. Visit `here <../refs/gtool.html>`_ for more information about gtool command line tool.

* Find account details from the website: http://testnet.fractalblock.com.

Deploy node without mining
------------------------------------------
Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    . setenv.sh
    gftl --testnet --rpc --datadir data --unlock [mypassword]

.. Note::   [mypassword] is the password when you create your account. Visit `here <../refs/gtool.html>`_ for more information about gtool command line tool.

Start another terminal to check status. Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    . setenv.sh
    gtool block --rpc [rpc address] --height 100 query

Then you can get the block detail from the terminal output with height 100.

.. Note::  Local node [rpc address] is http://127.0.0.1:8545. Visit `here <../refs/gtool.html>`_ for more information about gtool command line tool.

Deploy miner node
------------------------------------------
1. First, you must check your account stake balance. Since Fractal is powered by the proof-of-stake consensus protocol, you must hold stakes to start mining.
2. Register mining keys. Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    . setenv.sh
    gtool keys --rpc [rpc address] --keys data/keys --pass [mypassword] --chainid [chainid] regminingkey

.. Note::  Local node [rpc address] is http://127.0.0.1:8545. [mypassword] is the password when you create your account. [chainid] is 2 for testnet. Visit `here <../refs/gtool.html>`_ for more information about gtool command line tool.

3. Start miner node. Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    . setenv.sh
    gftl --rpc --testnet --datadir data --unlock [mypassword] --mine

.. Note::   [mypassword] is the password when you create your account. Visit `here <../refs/gtool.html>`_ for more information about gtool command line tool.

Initiate transaction
------------------------------------------
Transfer Token
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
Run these commands in terminal: 

.. code-block:: bash

    cd ~/fractal-test
    . setenv.sh
    gtool tx --rpc [rpc address] --keys data/keys --password [mypassword] --to [account address] --value [number] --chainid [chainid] send

.. Note::  Local node [rpc address] is http://127.0.0.1:8545. [mypassword] is the password when you create your account. [account address] should be a valid account address. [number] is the token amount you want to transfer. [chainid] is 2 for testnet. Visit `here <../refs/gtool.html>`_ for more information about gtool command line tool.


