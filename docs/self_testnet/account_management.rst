Account-Management 
-------------------
After installing your version of go-fractal, you can generate your account for mining and send-transaction.
account-management includes ``three`` steps, you can choose one according to actual requirement:

1. generate account :ref:`detail <generate-account-label>`.

2. list account :ref:`detail <list-account-label>`.

3. lookup balance :ref:`detail <lookup-balance-label>`.



.. _generate-account-label:

generate account 
^^^^^^^^^^^^^^^^^
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
    //make a directory , you may want to create more directories as your pleasure.
    $ mkdir data
    $ mkdir data1

``pass`` argument for (data/keys and  data1/keys) should be the same ,
we will use this feature in :ref:`generate allocation <generate-allocation-file-label>`
::
    //--keys is where to put the keys , --pass is your password ,remember to set your own password
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
The newest format of the three keyfiles are: ``account.json``, ``packer.json``, ``xxxx.mk.json`` . Note that all keys are stored in 
encryption. ``packer.json`` is not used unless you are selected as a packer,  ``xxxx.mk.json`` is your miner key.

.. _list-account-label:

list account
^^^^^^^^^^^^^^
If you want to look through informations like  account address ,miner address ... etc,you can use this command:
::
    $ gtool keys --keys data/keys --pass 666 list
    Packer Key Address: 0x24c6baa88a465e9a6a64faca0725ebb4f87414e5
    Packer Public Key: 0x04511a4aeda4d6fc3855f67df8b62cd22d008af37f332578cb198dcaa93a09fae2ef2f88a30bf0fa3e96724786e4aa99c6f2a47a403ed18edbd05d52f8d4b1a2cd
    Account Key Address: 0x24c6baa88a465e9a6a64faca0725ebb4f87414e5
    Mining Key Address: 0x24c6baa88a465e9a6a64faca0725ebb4f87414e5
    Mining Public Key: 0x8a21ce8992d6f32450f95dfbea26fa4bb45222d2395a537ee1c079e049cb16cc04f703ba84d0f9df120ce1e45e1868b970bcb4deecc531a1d5634b8de6fea232637cc37b369891ce774a2fe6084f14e110734e97d65a15fb3ebbdc706ac0c21f54bbb1098e409d3e997823d9ea6cf1c0f055de91ea02b08653b90859c9a40c19
**WARNING** data/keys is your key directory , ``--pass`` is your password


.. _lookup-balance-label:

lookup balance
^^^^^^^^^^^^^^^
balance information is store on chain ,so you need to assign a rpc connection.
::
    $ gtool state --rpc http://127.0.0.1:8545 --addr 0x24c6baa88a465e9a6a64faca0725ebb4f87414e5 account
    t=2019-07-02T18:48:36+0800 lvl=info msg="get head block ok" height=23 round=1562064515 hash=0x1c36dc5132a024ae6afffddd02f43b36850c35bcd8fd2f09d45ff3ff730aa3d5
    t=2019-07-02T18:48:36+0800 lvl=info msg="get balance ok" addr=0x24c6Baa88a465E9a6A64fACa0725eBb4F87414e5 balance=500211000000000
    t=2019-07-02T18:48:36+0800 lvl=info msg="get code ok" addr=0x24c6Baa88a465E9a6A64fACa0725eBb4F87414e5 len=0 code=0x
    t=2019-07-02T18:48:36+0800 lvl=info msg="get owner ok" addr=0x24c6Baa88a465E9a6A64fACa0725eBb4F87414e5 owner=0x0000000000000000000000000000000000000000

**WARNING** rpc is your node connection, addr is the account you want to check balance, if you don't know it ,you can 
use :ref:`list account <list-account-label>` command to get addr




