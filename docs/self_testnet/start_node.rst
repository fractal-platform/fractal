Start-Node 
---------------
includes ``four`` steps

1. download files :ref:`detail <download-file-label>`.

2. generate account :ref:`detail <generate-account-label>`.

3. generate allocation file :ref:`detail <generate-allocation-file-label>`.

4. start node :ref:`detail <start-node-label>`.


.. _download-file-label:

download files
^^^^^^^^^^^^^^^^^

There are four kind of installation-files,follow the appropriate link below to find installation instructions for your platform.

-    `installation_mac <../installation_mac.html>`_.

-   `installation_ubuntu <../installation_ubuntu.html>`_.

-    `installation_centos <../installation_centos.html>`_.

.. _generate-account-label:

generate account
^^^^^^^^^^^^^^^^^

After downloading files from server,we have to generate account for mining and transaction.

**WARNING**
Remember your password. The password is used for encryption of ``miner key`` , ``account key`` and ``packer key``.
Note that, **password is different from keys** ,it is used to protect keys, so keys can be transfered in encryption, 
There are three kind of keys:one is for mining and one is for account and one is for packer.
Packer key is not used for current time,it is used if you are packer,so just ignore it.

If you forget your password ,you will not be able to get your money back ,mine block with your existing balance , or send transactions any more.

**Repeat: Backup your password**

The fractal CLI ``gtool`` is used like this :
::
    $ gtool <main-command> [options...] [arguments...] <sub-command>

Generate account command is :
::
    //make two directories because we want to transfer balance from A to B , you may want to create more directories as your pleasure.
    $ mkdir data
    $ mkdir data1

``pass`` argument below for (data/keys and  data1/keys) should be the same ,we will use this feature in :ref:`generate allocation <generate-allocation-file-label>`
::
    //--keys is where to put the keys ,you don't need to make it if it is not created , --pass is your password ,remember to set your own password
    $ gtool keys --keys data/keys --pass 666 newkeys
    New Account Key Address: 0x24c6baa88a465e9a6a64faca0725ebb4f87414e5
    New Mining Key Address: 0x24c6baa88a465e9a6a64faca0725ebb4f87414e5
    New Mining Public Key: 0x8a21ce8992d6f32450f95dfbea26fa4bb45222d2395a537ee1c079e049cb16cc04f703ba84d0f9df120ce1e45e1868b970bcb4deecc531a1d5634b8de6fea232637cc37b369891ce774a2fe6084f14e110734e97d65a15fb3ebbdc706ac0c21f54bbb1098e409d3e997823d9ea6cf1c0f055de91ea02b08653b90859c9a40c19
    New Packer Key Address: 0x24c6baa88a465e9a6a64faca0725ebb4f87414e5
   
    $ gtool keys --keys data1/keys --pass 666 newkeys
    New Account Key Address: 0xc402b930dbe2a2fec29dc4699dc0c17f19805949
    New Mining Key Address: 0xc402b930dbe2a2fec29dc4699dc0c17f19805949
    New Mining Public Key: 0x866c641dca6652119d2c2b9e06d30c08264ffc94e0bfa9694df54a8989939c9b5f41cb13f6e01373fa2e956ba5a388084024d399bb36ccd8438770a8971432556851804a0ccf2d8f0758aecf7b103802d8673f7c157fdcde39d3febc8ab18c65881b4eeb3f4db30ec0ed41280ea92d15494b604d0f56012706e26cfa8c7713fe
    New Packer Key Address: 0xc402b930dbe2a2fec29dc4699dc0c17f19805949

You can see three kind of keys in ``data/keys`` directory.
**WARNING** We want to transfer balance from A(``data/``) user to B(``data1``) user later ,so we create two accounts.


.. _generate-allocation-file-label:

generate allocation
^^^^^^^^^^^^^^^^^^^
This is a must step of self-testnet, if we want to mine, we need stakes(account balance),original stakes is allocated in allocation-file.This step is to create
an allocation-file.

Generate allocation command is:
::
    $ gtool gstate --pass 666 gen
    scan folder: data
    scan folder: data1

**WARNING** 
``--pass`` is your password, but for testnet environment,password for data/keys and password for data1/keys need to be the same,we would improve this later on.
This command scans current directory to check ``keys`` directory,and generate ``genesis_alloc.json`` file ,you need to use this file later on.
So you need to enter ``data``'s or ``data1``'s parent directory, so as to scan it.

.. _start-node-label:

start node
^^^^^^^^^^^
This the final step of start-node, after this step, one fractal node will be running.
start node command is:
::
    $ gftl --config test.toml --genesisAlloc genesis_alloc.json --rpc --rpcport 8545 --datadir data --port 30303 --pprof --pprofport 6060 --verbosity 3 --mine --unlock 666
**WARNING** ``test.toml`` is chain config file,``genesis_alloc.json`` is balance allocation file ,``rpc port`` is an http server to receive message from user,
``data`` is your mining data directory, ``mine`` is mining-open flag , ``unlock`` is your password.

If you want to start a backgroud-node , you can use ``nohup`` command.
::
    $ nohup gftl --config test.toml --genesisAlloc genesis_alloc.json --rpc --rpcport 8545 --datadir data --port 30303 --pprof --pprofport 6060 --verbosity 3 --mine --unlock 666 > gftl.log &

If you want start a node and connect to a known one ,use ``enode``,below is data1 node connects to data node,remember to change ports if you run data1 node in the same physical machine:
::
    $ gftl --config test.toml --genesisAlloc genesis_alloc.json --rpc --rpcport 8546 --datadir data1 --port 30304 --pprof --pprofport 6061 --verbosity 3 --mine --unlock 666 --bootnodes enode://2b36b97ea62b8ff41011223ff0720db7e468500e2aa3253668f13a9ecd15fbbd5c1ccce8252712c063cd166f1f7be95747574cf6a68d9726a3fad62cdb40f34e@127.0.0.1:30303

You can get ``enode`` using this command:
::  
    $ gtool admin --rpc http://127.0.0.1:8545 enode
    enode://83afd5c4e7167257d1e0b161d54c1f2a581f948472912a33320df87e845fd13831e6242ab327ee489b92254468a55e9df5863c5bf5218b42f9aa039ff3b585be@10.1.1.168:30303

**WARNING** If you want to check one node's enode, you need to assign rpc server.




