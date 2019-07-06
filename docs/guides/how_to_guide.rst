How To Guides
=============

In this chapter we will discard scripts because it is not flexible,
maybe you want to start more fractal nodes, send transaction etc.
Here we introduce you a few common ``How To``  step-by-step. 
You’ll learn:

- How to Start a ``PrivateNetwork`` step by step.
- How to Start a ``TestNetwork`` step by step.
- How to Send a Transaction step by step.
- How to Deploy a Smart Contract.

| **NOTE**
| The fractal CLI ``gtool`` is used like this,other commands are the same:
|    ``$ gtool <main-command> [options...] [arguments...] <sub-command>``
| It contains ``main-command`` and ``sub-command``.


How to Start a **PrivateNetwork** step by step
-----------------------------------------------------
Make sure you have a clean directory which only contains original fractal files. 

1. set environment variables

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


2. make directories to store keys and chaindata

.. code-block:: bash 

    //make two directories because we want to transfer balance from A to B , you may want to create more directories as your pleasure.
    $ mkdir data1
    $ mkdir data2
    
3. generate account , password after ``--pass`` of ``data1/keys`` and ``data2/keys`` should be the same

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

You can see three kind of keys in ``data1/keys`` and ``data2/keys`` directories.

4. generate allocation

::

    $ ./gtool gstate --pass 666 gen
    scan folder: data1
    scan folder: data2

``gstate`` scans current directory to check ``keys`` directory, and generate ``genesis_alloc.json`` file.

5. start nodes, ``data2`` node connects ``data1`` node using ``enode`` argument

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


**WARNNG** The second ``./gtool admin`` command can query ``enode`` which is used in the third command, you must assign ``--rpc`` server to get ``enode``, and you must change the third ``nohup`` command's ``enode`` argument.
Nodes may fail if the ports are in use : ``rpcport`` , ``port`` , ``pprofport`` , you should change them, for example: adding 1 to the port number.


How to Start a **TestNetwork** step by step
-----------------------------------------------------
Make sure you have a clean directory which only contains original fractal files. 

1. set environment variables

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

2. make directories to store keys and chaindata

.. code-block:: bash 

    $ mkdir -p data/keys/
    

You can see three kind of keys in ``data/keys`` directory.

3. start node

**If your operate on macOS**

.. code-block:: bash 

    $ nohup ./gftl --testnet --rpc --rpcport 8545 --datadir data --port 60001 --pprof --pprofport 6061 --verbosity 3 --mine --unlock 666 > gftl.log &

**If you operate on ubuntu or centos**

.. code-block:: bash 

    $ nohup ./gftl --testnet --rpc --rpcport 8546 --datadir data --port 60001 --pprof --pprofport 6061 --verbosity 3 --mine --unlock 666 > gftl.log 2>&1 &


**WARNNG** Nodes may fail if the ports are in use : ``rpcport`` , ``port`` , ``pprofport`` , you should change them, for example: adding 1 to the port number.

**NOTE: If you want to start mining for yourself, go on reading, otherwise you can stop here.**


How to Start Mining in Test Network
-----------------------------------------------------

Step 1. fetch wallet application from https://github.com/fractal-platform/wallet/releases

Step 2. create account in wallet.

Step 3. apply stake from official site, or ask your friend to transfer stake to you.

Step 4. start local node to join fractal test network.

Step 5. connect to your local node rpc in wallet.

Step 6. click ``register miner`` in wallet, and you will start mining on local node.


How to Send a Transaction step by step
-----------------------------------------------------
Once you have started a **TestNetwork** or **PrivateNetwork**, you can send transactions

.. code-block:: bash 

    $  gtool tx --rpc http://127.0.0.1:8545 --to 0xc402b930dbe2a2fec29dc4699dc0c17f19805949  --chainid 999 --keys data/keys --pass 666 send
    t=2019-07-02T19:35:12+0800 lvl=info msg="get nonce ok" nonce=0
    t=2019-07-02T19:35:12+0800 lvl=info msg="send tx success" hash=0x823e7dde4a4a68fad223beaf47124deeec0534a81a838add639b2a9374ed3ca4
    t=2019-07-02T19:35:14+0800 lvl=info msg="recv tx rsp" from=0xDc19ab8A51Ac78eb99392262e26681d64ba66317 nonce=0 hash=0x823e7dde4a4a68fad223beaf47124deeec0534a81a838add639b2a9374ed3ca4 to=0xC402B930dBe2a2FEc29dC4699DC0C17F19805949 receipt=<nil>

**WARNNG** you need to change ``rpc`` url , if your node address is not ``http://127.0.0.1:8545`` , but if you run ``start_private.sh`` or ``start_testnet.sh`` to startup nodes, the ``rpc`` url is default to 
``http://127.0.0.1:8545``; the ``to`` argument is the address you want to transfer balance to, you can change it. If you don't know the ``to`` address,
you can use  ``gtool keys --keys data/keys --pass 666 list`` to find the local address.


How to Deploy a Smart Contract
-----------------------------------------------------
Smart Contract steps are not expanded here, go `smart contract <https://fractal-cdt.readthedocs.io/en/v0.1.x/index.html>`_ to get more information.



