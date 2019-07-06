Usage
=========

This chapter covers all commands that appear in other chapters and explains how they work.
We list them here:

- gtool keys 
- gtool gstate
- gtool admin
- gtool state
- gtool tx
- gftl 
- wasmtest exec

| The fractal CLI ``gtool`` is used like this, other commands are the same:
| ``$ gtool <main-command> [options...] [arguments...] <sub-command>``
| It contains ``main-command`` and ``sub-command``.


gtool keys
--------------
It includes **generate account** and  **list account**.

generate account 
'''''''''''''''''
**WARNING**
Remember your password. The password is used for encryption of ``miner key`` , ``account key`` and ``packer key``.
Note that, **password is different from keys** ,it is used to protect keys, so keys can be transfered in encryption. 
There are three kind of keys: one is for mining, one is for account and one is for packer.
Packer key is used if you are a packer,so just ignore it.

If you forget your password ,you will not be able to get your money back, mine block with your existing balance , or send transactions any more.

**Repeat: Backup your password**

Generate account command is :

.. code-block:: bash 

    $ gtool keys --keys data/keys --pass 666 newkeys
    New Account Key Address: 0x24c6baa88a465e9a6a64faca0725ebb4f87414e5
    New Mining Key Address: 0x24c6baa88a465e9a6a64faca0725ebb4f87414e5
    New Mining Public Key: 0x8a21ce8992d6f32450f95dfbea26fa4bb45222d2395a537ee1c079e049cb16cc04f703ba84d0f9df120ce1e45e1868b970bcb4deecc531a1d5634b8de6fea232637cc37b369891ce774a2fe6084f14e110734e97d65a15fb3ebbdc706ac0c21f54bbb1098e409d3e997823d9ea6cf1c0f055de91ea02b08653b90859c9a40c19
    New Packer Key Address: 0x24c6baa88a465e9a6a64faca0725ebb4f87414e5

| ``--keys`` is which directory to put the keys 
| ``--pass`` is your password ,remember to set your own password

You can see three kind of keys in ``data/keys`` directory now.
The newest format of the three keyfiles are: ``account.json``, ``packer.json``, ``xxxx.mk.json`` . Note that all keys are stored in 
encryption. ``packer.json`` is not used unless you are selected as a packer,  ``xxxx.mk.json`` is your miner key.


list account
'''''''''''''
If you want to look through informations like  account address ,miner address ... etc, you can use this command:

.. code-block:: bash 

    $ gtool keys --keys data/keys --pass 666 list
    Packer Key Address: 0x24c6baa88a465e9a6a64faca0725ebb4f87414e5
    Packer Public Key: 0x04511a4aeda4d6fc3855f67df8b62cd22d008af37f332578cb198dcaa93a09fae2ef2f88a30bf0fa3e96724786e4aa99c6f2a47a403ed18edbd05d52f8d4b1a2cd
    Account Key Address: 0x24c6baa88a465e9a6a64faca0725ebb4f87414e5
    Mining Key Address: 0x24c6baa88a465e9a6a64faca0725ebb4f87414e5
    Mining Public Key: 0x8a21ce8992d6f32450f95dfbea26fa4bb45222d2395a537ee1c079e049cb16cc04f703ba84d0f9df120ce1e45e1868b970bcb4deecc531a1d5634b8de6fea232637cc37b369891ce774a2fe6084f14e110734e97d65a15fb3ebbdc706ac0c21f54bbb1098e409d3e997823d9ea6cf1c0f055de91ea02b08653b90859c9a40c19

| ``data/keys`` is your key directory 
| ``--pass`` is your password


gtool gstate
--------------
This is a must step of ``Private-Network`` if you start node step-by-step, if you want to mine,
you need stakes(account balance), original stakes are allocated in allocation-file. This command is to create
an allocation-file.

Generate allocation command is:

.. code-block:: bash 

    $ gtool gstate --pass 666 gen
    scan folder: data
    scan folder: data1

| ``--pass`` is your password, but for testnet environment, password for data/keys and password for data1/keys need to be the same, we would improve this later on.

This command scans current directory to check ``keys`` directory,and generate ``genesis_alloc.json`` file ,you need to use this file later on.
So you need to enter ``data``'s and ``data1``'s parent directory, so as to scan it.


gtool admin
--------------
This is a command to get ``enode``, ``enode`` is an argument you can use to connect to other nodes

.. code-block:: bash 

    $ gtool admin --rpc http://127.0.0.1:8545 enode
    enode://83afd5c4e7167257d1e0b161d54c1f2a581f948472912a33320df87e845fd13831e6242ab327ee489b92254468a55e9df5863c5bf5218b42f9aa039ff3b585be@10.1.1.168:30303

**WARNING** If you want to check one node's enode, you need to assign rpc server.


gtool state
--------------
This command is to get your left balance on fractal chain. Balance information is stored on chain ,so you need to assign a rpc connection.

.. code-block:: bash 

    $ gtool state --rpc http://127.0.0.1:8545 --addr 0x24c6baa88a465e9a6a64faca0725ebb4f87414e5 account
    t=2019-07-02T18:48:36+0800 lvl=info msg="get head block ok" height=23 round=1562064515 hash=0x1c36dc5132a024ae6afffddd02f43b36850c35bcd8fd2f09d45ff3ff730aa3d5
    t=2019-07-02T18:48:36+0800 lvl=info msg="get balance ok" addr=0x24c6Baa88a465E9a6A64fACa0725eBb4F87414e5 balance=500211000000000
    t=2019-07-02T18:48:36+0800 lvl=info msg="get code ok" addr=0x24c6Baa88a465E9a6A64fACa0725eBb4F87414e5 len=0 code=0x
    t=2019-07-02T18:48:36+0800 lvl=info msg="get owner ok" addr=0x24c6Baa88a465E9a6A64fACa0725eBb4F87414e5 owner=0x0000000000000000000000000000000000000000

| ``--rpc`` is your node connection
| ``--addr`` is the account you want to check balance, if you don't know it ,you can use `list account` command to get local addr or check the wallet.


gtool tx
--------------
You can send transactions, we only put ``transfer balance from A user to B user`` here, but for smart contract use , go `smart contract <xxx>`_.

Send transaction command is :

.. code-block:: bash 

    $  gtool tx --rpc http://127.0.0.1:8545 --to 0xc402b930dbe2a2fec29dc4699dc0c17f19805949  --chainid 999 --keys data/keys --pass 666 send
    t=2019-07-02T19:35:12+0800 lvl=info msg="get nonce ok" nonce=0
    t=2019-07-02T19:35:12+0800 lvl=info msg="send tx success" hash=0x823e7dde4a4a68fad223beaf47124deeec0534a81a838add639b2a9374ed3ca4
    t=2019-07-02T19:35:14+0800 lvl=info msg="recv tx rsp" from=0xDc19ab8A51Ac78eb99392262e26681d64ba66317 nonce=0 hash=0x823e7dde4a4a68fad223beaf47124deeec0534a81a838add639b2a9374ed3ca4 to=0xC402B930dBe2a2FEc29dC4699DC0C17F19805949 receipt=<nil>

| ``--rpc`` is the chain server
| ``--to`` is the balance receiver
| ``--chainid`` you must assign ``chainid`` here according to your ``test.toml``, ``chainid`` is the flag to distinguish testnet environment from main-net environment.
| ``--keys`` is your key directory 
| ``--pass`` is your password

Transaction amount is fixed to 1 ``ftl``,so you don't need to assign it .

gftl 
--------------
This the final step of start-node, after this step, one fractal node will be running.
Start node command is:

.. code-block:: bash 

    $ gftl --config test.toml --genesisAlloc genesis_alloc.json --rpc --rpcport 8545 --datadir data --port 30303 --pprof --pprofport 6060 --verbosity 3 --mine --unlock 666

| ``--config`` is chain config file
| ``--genesisAlloc`` is balance allocation file 
| ``--rpcport`` is a http server to receive messages from user
| ``--data`` is your mining data directory
| ``--mine`` is mining-open flag 
| ``--unlock`` is your password

If you want to start a backgroud-node , you can use ``nohup`` command.

.. code-block:: bash 

    $ nohup gftl --config test.toml --genesisAlloc genesis_alloc.json --rpc --rpcport 8545 --datadir data --port 30303 --pprof --pprofport 6060 --verbosity 3 --mine --unlock 666 > gftl.log &

If you want start a node and connect to a known one ,use ``enode``, below is ``data1`` node connects to ``data`` node, remember to change ports if you run ``data1`` node in the same physical machine:

.. code-block:: bash 

    $ gftl --config test.toml --genesisAlloc genesis_alloc.json --rpc --rpcport 8546 --datadir data1 --port 30304 --pprof --pprofport 6061 --verbosity 3 --mine --unlock 666 --bootnodes enode://2b36b97ea62b8ff41011223ff0720db7e468500e2aa3253668f13a9ecd15fbbd5c1ccce8252712c063cd166f1f7be95747574cf6a68d9726a3fad62cdb40f34e@127.0.0.1:30303

You can get ``enode`` using ``gtool admin`` command:

.. code-block:: bash 

    $ gtool admin --rpc http://127.0.0.1:8545 enode
    enode://83afd5c4e7167257d1e0b161d54c1f2a581f948472912a33320df87e845fd13831e6242ab327ee489b92254468a55e9df5863c5bf5218b42f9aa039ff3b585be@10.1.1.168:30303

**WARNING** If you want to check one node's enode, you need to assign his rpc server.


wasmtest exec
--------------
This command lets you test your smart contract to check whether it is wrong or not.
 
test command is:

.. code-block:: bash 

    $ wasmtest --wasm hello.wasm --abi hello.abi --action hi --args '["Alice"]' exec

| ``--wasm`` is your wasm file path
| ``--abi`` is your abi file path
| ``--action`` is your smart contract action name
| ``--args`` is your action args

**WARNING** If you don't have hello.wasm or hello.abi, go to `smart contract <https://fractal-cdt.readthedocs.io/en/v0.1.x/index.html>`_ to see how to generate them.