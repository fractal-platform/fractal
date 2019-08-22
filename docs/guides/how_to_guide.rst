How To Guides
=============

In this chapter we will dive into more details on Fractal. This chapter introduce commands that give you flexibility and
help you start more fractal nodes and initiate transaction etc. Here we introduce you a few common ``How To`` step-by-step.
You’ll learn:

- How to Start a ``PrivateNetwork`` step by step.
- How to Start a ``TestNetwork`` step by step.
- How to Start Mining in ``TestNetwork``
- How to Initiate a Transaction step by step.
- How to Deploy a Smart Contract.

| **NOTE**
| The fractal CLI ``gtool`` syntax is shown as follows:
|    ``$ gtool <main-command> [options...] [arguments...] <sub-command>``
| the syntax contains ``main-command`` and ``sub-command``. Other fractal commands follow similar syntax and notation.


How to Start a **PrivateNetwork** step by step
-----------------------------------------------------

1. unzip downloaded release file

.. code-block:: bash

    $ tar -zxvf fractal-bin.<OS>.v0.1.0.tar  -C .

2. cd fractal-bin.<OS>.v0.1.0

.. code-block:: bash

    $ cd fractal-bin.<OS>.v0.1.0

3. set environment variables

**If your operate on macOS**

.. code-block:: bash 

    // assume /path/to/fractal-bin is the path you decompress fractal-bin.macos.v0.1.0.tar.gz
    $ export DYLD_LIBRARY_PATH=/path/to/fractal-bin
    $ export PATH=$PATH:/path/to/fractal-bin

**If you operate on ubuntu or centos**

.. code-block:: bash 

    // assume /path/to/fractal-bin is the path you decompress fractal-bin.ubuntu.v0.1.0.tar.gz
    $ export LD_LIBRARY_PATH=/path/to/fractal-bin
    $ export PATH=$PATH:/path/to/fractal-bin


4. create directories to store keys and chaindata

.. code-block:: bash 

    //make at least two directories since we want to transfer balance from A to B.
    $ mkdir data1
    $ mkdir data2
    
5. generate account, password follows ``--pass`` of ``data1/keys`` and ``data2/keys`` should be identical

.. code-block:: bash 

    $ ./gtool keys --keys data1/keys --pass 666 newkeys
    New Account Key Address: 0x24c6baa88a465e9a6a64faca0725ebb4f87414e5
    New Mining Key Address: 0x24c6baa88a465e9a6a64faca0725ebb4f87414e5
    New Mining Public Key: 0x8a21ce8992d6f32450f95dfbea26fa4bb45222d2395a537ee1c079e049cb16cc04f703ba84d0f9df120ce1e45e1868b970bcb4deecc531a1d5634b8de6fea232637cc37b369891ce774a2fe6084f14e110734e97d65a15fb3ebbdc706ac0c21f54bbb1098e409d3e997823d9ea6cf1c0f055de91ea02b08653b90859c9a40c19
    New Packer Key Address: 0x24c6baa88a465e9a6a64faca0725ebb4f87414e5

    $ ./gtool keys --keys data2/keys --pass 666 newkeys
    New Account Key Address: 0xc402b930dbe2a2fec29dc4699dc0c17f19805949
    New Mining Key Address: 0xc402b930dbe2a2fec29dc4699dc0c17f19805949
    New Mining Public Key: 0x866c641dca6652119d2c2b9e06d30c08264ffc94e0bfa9694df54a8989939c9b5f41cb13f6e01373fa2e956ba5a388084024d399bb36ccd8438770a8971432556851804a0ccf2d8f0758aecf7b103802d8673f7c157fdcde39d3febc8ab18c65881b4eeb3f4db30ec0ed41280ea92d15494b604d0f56012706e26cfa8c7713fe
    New Packer Key Address: 0xc402b930dbe2a2fec29dc4699dc0c17f19805949

Now, you will see three kinds of keys in ``data1/keys`` and ``data2/keys`` directories.

6. generate allocation

::

    $ ./gtool gstate --pass 666 gen
    scan folder: data1
    scan folder: data2

``gstate`` scans current directory, finds ``keys`` directory and generates ``genesis_alloc.json`` file.

7. start nodes

The following command allows ``data2`` node connects ``data1`` node using ``enode`` argument

**If your operate on macOS**

.. code-block:: bash 

    $ nohup ./gftl --config test.toml --genesisAlloc genesis_alloc.json --rpc --rpcport 8545 --datadir data1 --port 50000 --pprof --pprofport 6060 --verbosity 3 --mine --unlock 666 > gftl1.log &
    $ ./gtool admin --rpc http://127.0.0.1:8545 enode
    $ nohup ./gftl --config test.toml --genesisAlloc genesis_alloc.json --rpc --rpcport 8546 --datadir data2 --port 50001 --pprof --pprofport 6061 --verbosity 3 --mine --unlock 666 --bootnodes enode://2b36b97ea62b8ff41011223ff0720db7e468500e2aa3253668f13a9ecd15fbbd5c1ccce8252712c063cd166f1f7be95747574cf6a68d9726a3fad62cdb40f34e@127.0.0.1:50000 > gftl2.log &

**If you operate on ubuntu or centos**

.. code-block:: bash 

    $ nohup ./gftl --config test.toml --genesisAlloc genesis_alloc.json --rpc --rpcport 8545 --datadir data --port 50000 --pprof --pprofport 6060 --verbosity 3 --mine --unlock 666 > gftl1.log 2>&1 &
    $ ./gtool admin --rpc http://127.0.0.1:8545 enode
    $ nohup ./gftl --config test.toml --genesisAlloc genesis_alloc.json --rpc --rpcport 8546 --datadir data1 --port 50001 --pprof --pprofport 6061 --verbosity 3 --mine --unlock 666 --bootnodes enode://2b36b97ea62b8ff41011223ff0720db7e468500e2aa3253668f13a9ecd15fbbd5c1ccce8252712c063cd166f1f7be95747574cf6a68d9726a3fad62cdb40f34e@127.0.0.1:30303 > gftl2.log 2>&1 &


**WARNNG** The second command ``./gtool admin`` queries ``enode``, which is later used in the third command. You must assign ``--rpc`` server to reach ``enode``, and you must change the third ``nohup`` command's ``enode`` argument.
If you see error like port ``rpcport`` , ``port`` , ``pprofport`` is already in use, please change the port number.


How to Start a **TestNetwork** step by step
-----------------------------------------------------

1. unzip downloaded release file

.. code-block:: bash

    $ tar -zxvf fractal-bin.<OS>.v0.1.0.tar  -C .

2. cd fractal-bin.<OS>.v0.1.0

.. code-block:: bash

    $ cd fractal-bin.<OS>.v0.1.0

3. set environment variables

**If your operate on macOS**

.. code-block:: bash 

    // /path/to/fractal-bin is the path you decompress fractal-bin.macos.v0.1.0.tar.gz 
    $ export DYLD_LIBRARY_PATH=/path/to/fractal-bin
    $ export PATH=$PATH:/path/to/fractal-bin

**If you operate on ubuntu or centos**

.. code-block:: bash 

    // /path/to/fractal-bin is the path you decompress fractal-bin.ubuntu.v0.1.0.tar.gz
    $ export LD_LIBRARY_PATH=/path/to/fractal-bin
    $ export PATH=$PATH:/path/to/fractal-bin

4. create directories to store keys and chaindata

.. code-block:: bash 

    $ mkdir -p data/keys/
    

Now, you will see three kinds of keys in ``data/keys`` directory.

5. start node

**If your operate on macOS**

.. code-block:: bash 

    $ nohup ./gftl --testnet --rpc --rpcport 8545 --datadir data --port 60001 --pprof --pprofport 6061 --verbosity 3 --mine --unlock 666 > gftl.log &

**If you operate on ubuntu or centos**

.. code-block:: bash 

    $ nohup ./gftl --testnet --rpc --rpcport 8546 --datadir data --port 60001 --pprof --pprofport 6061 --verbosity 3 --mine --unlock 666 > gftl.log 2>&1 &


**WARNNG** If you see error like port ``rpcport`` , ``port`` , ``pprofport`` is already in use, please change the port number.


**NOTE: The next section introduces how to start mining. If you prefer not to do so, you can skip it.**


How to Start Mining in Test Network
-----------------------------------------------------

Step 1. download wallet application from https://github.com/fractal-platform/wallet/releases

Step 2. create account in wallet.

Step 3. apply stake from official site, or ask someone to transfer stake to you.

Step 4. start local node to join fractal test network.

Step 5. connect to your local node rpc in wallet.

Step 6. click ``register miner`` in wallet, and you will start mining on local node.


How to Initiate a Transaction step by step
-----------------------------------------------------
Once you have started a **TestNetwork** or **PrivateNetwork**, you can initiate transactions

.. code-block:: bash 

    $  gtool tx --rpc http://127.0.0.1:8545 --to 0xc402b930dbe2a2fec29dc4699dc0c17f19805949  --chainid 999 --keys data/keys --pass 666 send
    t=2019-07-02T19:35:12+0800 lvl=info msg="get nonce ok" nonce=0
    t=2019-07-02T19:35:12+0800 lvl=info msg="send tx success" hash=0x823e7dde4a4a68fad223beaf47124deeec0534a81a838add639b2a9374ed3ca4
    t=2019-07-02T19:35:14+0800 lvl=info msg="recv tx rsp" from=0xDc19ab8A51Ac78eb99392262e26681d64ba66317 nonce=0 hash=0x823e7dde4a4a68fad223beaf47124deeec0534a81a838add639b2a9374ed3ca4 to=0xC402B930dBe2a2FEc29dC4699DC0C17F19805949 receipt=<nil>

**WARNNG** If you run ``start_private.sh`` or ``start_testnet.sh`` to startup nodes, the ``rpc`` url is by default set to
``http://127.0.0.1:8545``; hence if your node address is not ``http://127.0.0.1:8545`` you need to change ``rpc`` url accordingly.
The ``to`` argument is the transfer recipient address, you should change it. If you don't know the ``to`` address,
you can use  ``gtool keys --keys data/keys --pass 666 list`` to find a local address.


How to Deploy a Smart Contract
-----------------------------------------------------
Smart Contract steps are introduced here `smart contract <https://fractal-cdt.readthedocs.io/en/v0.1.x/index.html>`_ .



