Managing your account
-----------------------
After installing your version of go-fractal, you can generate your account for mining or wallet.

**WARNING**
Remember your password. The password is used for encryption of Mining.
Note that, **password is different from Keys** , and there are two kinds of keys:one is for mining and the other is for transaction.

If you forget your password ,you will not be able to get your money back ,mine block with your existing balance , or send transactions any more.

**Repeat: Backup your password**

The fractal CLI ``gtool`` provides account management via the ``keys`` command:

::

    $ gtool [options...] [arguments...] keys <command>


Manage accounts lets you create new accounts, list all existing accounts, change your password.

It supports interactive mode, when you are prompted for password as well as non-interactive(**it means you can use it in scripted file like shell**) mode where passwords are supplied via a given password file.
Non-interactive mode is only meant for scripted use on test networks or known safe environments.

Make sure you remember the password you gave when creating a new account (with new, update). Without it you are not able to unlock your account.

Note that exporting your key in unencrypted format is NOT supported.

Keys are stored under ``<DATADIR>/keystore``. Make sure you backup your keys regularly! If a custom datadir and keystore option are given the keystore
option takes preference over the datadir option.

The newest format of the two keyfiles are: ``UTC--<created_at UTC ISO8601>-<address hex>--X``. When `X= 0` ,it means the mining key,when `X= 1` ,it means the transaction key. The order of accounts when listing,
 is lexicographic, but as a consequence of the timestamp format, it is actually order of creation.

It is safe to transfer the entire directory or the individual keys between fractal nodes. Note that in case you are adding keys to your node from a different node,
the order of accounts may change. So make sure you do not rely on the index in your scripts or code snippets.

And again. **DO NOT FORGET YOUR PASSWORD**

.. code-block:: shell 

        COMMANDS:
            list    Print summary of existing accounts
            new     Create a new account
            update  Update an existing account

Examples
^^^^^^^^^^

Interactive use
^^^^^^^^^^^^^^^^^
creating an account 

.. code-block:: shell

    //create a directory
    $ mkdir data

    //make a new account
    $ gtool --keys data/keys --pass 666 keys newkeys
    Your new account is locked with a password. Please give a password. Do not forget this password.
    Passphrase:
    Repeat passphrase:
    Address: {197acc66644b33e8ae8c9dea4811bab84c510716}

Listing accounts in a custom ``datadir`` directory or ``datadir/keystore`` directory

.. code-block:: shell

    $ gftl --datadir data/ account list

    //another way is --keystore
    $ gftl --keystore data/keystore/ account list
    Account #0: {3a6deb5a0a041a0c139fa1d43f321c68e574a8a7}
    Account #0(miner key): {128: 81c2a2af1236d7eabe0405baf02fb38d6cadf7b55c22705d10653e72e97eb81a553b68423f0a404b6ab36f6f04a18d09db46ba383a4e3cda2e38d560cfd9bb903d45314ef78c1979656d07f5aad70b6f92666a92b6d8bdcb9e02f45b3830de0066d0240efff152b2ca2d351e743184b9cd2e4e249f491f26e4e37c3736cd5e11}
    Account #1: {c28a50f3af771c799297b3f8f2887b323ff6deed}
    Account #1(miner key): {128: 3639b3a48cfaa81c978ce456bbc6478a1a56c6910b556670ba007c053699b2427b14ef89e9ef6bf7df8acf8172e29afd4f06fe2bb5e6753371f0e9a0ed3d888119507d518f0fa7df66a09d39ab9c2693064aa3a19644086678ea28ca8065d563269f451e197405759aa9bdb8dba6c68ea18fbeacacba936c12abfd07fe472721}

Changing account password in a custom ``datadir`` directory or ``datadir/keystore`` directory

.. code-block:: shell

    $ gftl --datadir data/  account update c28a50f3af771c799297b3f8f2887b323ff6deed

    //use keystore
    $ gftl --keystore data/keystore  account update c28a50f3af771c799297b3f8f2887b323ff6deed
    Please enter the old password.
    Passphrase:
    Please give a new password. Do not forget this password.
    Passphrase:
    Repeat passphrase:
    
You can also specify multiple accounts at once.

.. code-block:: shell

    gftl --keystore data/keystore/  account update  91cd09230939cdbf0066c689f4b7f8c224e72c23 dac185bceccb81c64cf0af645d34d68df94fe275

Non-interactive use
^^^^^^^^^^^^^^^^^^^^^^

You supply a plaintext password file as argument to the ``--password`` flag. The data in the file consists of the raw characters of the password, followed by a single newline.

**Note**: Supplying the password directly as part of the command line is not recommended,
but you can always use shell trickery to get round this restriction.


.. code-block:: shell

    $ gftl --password pwd.txt --datadir data/ account new
    
**Warning:** If you use the password flag with a password file, best to make sure the file is not readable or even listable for anyone but you. You achieve this with:

.. code-block:: shell

        //path is a directory you make
        touch /path/pwd.txt
        chmod 700 /path/pwd

On the console, use:

.. code-block:: shell

    > personal.NewAccount()
    ... you will be prompted for a password ...

or

.. code-block:: shell

    > personal.newAccount("passphrase")


Listing accounts and checking balances

Listing your current accounts When using the console:

.. code-block:: shell 

    > gftl.accounts
    ["0x5afdd78bdacb56ab1dad28741ea2a0e47fe41331", "0x9acb9ff906641a434803efb474c96a837756287f"]


or via RPC:

.. code-block:: shell 

    Request
    $ curl -X POST --data '{"jsonrpc":"2.0","method":"eth_accounts","params":[],"id":1} http://127.0.0.1:8545'
    Result
    {
    "id":1,
    "jsonrpc": "2.0",
    "result": ["0x5afdd78bdacb56ab1dad28741ea2a0e47fe41331", "0x9acb9ff906641a434803efb474c96a837756287f"]
    }


Checking account balances

To check your the account balance:

.. code-block:: shell

    > rubanjs.fromWei(gftl.getBalance(gftl.coinbase), "gftl")
    6.5


Print all balances with a JavaScript function:

.. code-block:: shell

    function checkAllBalances() {
        var totalBal = 0;
        for (var acctNum in eth.accounts) {
            var acct = eth.accounts[acctNum];
            var acctBal = web3.fromWei(eth.getBalance(acct), "ether");
            totalBal += parseFloat(acctBal);
            console.log("  eth.accounts[" + acctNum + "]: \t" + acct + " \tbalance: " + acctBal + " ether");
        }
        console.log("  Total balance: " + totalBal + " ether");
    };

That can then be executed with:

.. code-block:: shell

    > checkAllBalances();
    eth.accounts[0]: 0xd1ade25ccd3d550a7eb532ac759cac7be09c2719 	balance: 63.11848 ether
    eth.accounts[1]: 0xda65665fc30803cb1fb7e6d86691e20b1826dee0 	balance: 0 ether
    eth.accounts[2]: 0xe470b1a7d2c9c5c6f03bbaa8fa20db6d404a0c32 	balance: 1 ether
    eth.accounts[3]: 0xf4dd5c3794f1fd0cdc0327a83aa472609c806e99 	balance: 6 ether
   
