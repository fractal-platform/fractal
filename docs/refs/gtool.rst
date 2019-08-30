Command Line Usage <gtool>
====================================
Help
------------------------------------
.. parsed-literal::
    gtool command [command options] subcommand

    VERSION:
       0.2.0-stable

    COMMANDS:
       admin   Manage Fractal Node
       block   Query Block
       gstate  Manage Fractal Genesis State
       keys    Manage Fractal Keys
       packer  Manage Fractal Packer
       state   Query Fractal State
       tx      Generate Transaction
       help    Shows a list of commands or help for one command

gtool admin
------------------------------------
Help
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
.. parsed-literal::
    NAME:
       gtool admin - Manage Fractal Node

    USAGE:
       gtool admin [command options] subcommand

    SUBCOMMANDS:
         info          Show Fractal Node Info
         enode         Show Fractal Node Enode Address
         genminingkey  Generate Mining Key fro Current Address

    OPTIONS:
       --rpc value   rpc service address
       --addr value  The address for keys
       --help, -h    show help

gtool block
------------------------------------
Help
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
.. parsed-literal::
    NAME:
       gtool block - Query Block

    USAGE:
       gtool block [command options] subcommand

    SUBCOMMANDS:
         query  Query Block Detail

    OPTIONS:
       --rpc value     rpc service address
       --height value  block height (default: 0)
       --bhash value   block hash
       --help, -h      show help

Example - Query Block with Height
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
Assume:
    * RPC server address is *http://127.0.0.1:8545*

.. code-block:: bash

    gtool block --rpc http://127.0.0.1:8545 --height 100 query

gtool gstate
------------------------------------
Help
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
.. parsed-literal::
    NAME:
       gtool gstate - Manage Fractal Genesis State

    USAGE:
       gtool gstate [command options] subcommand

    SUBCOMMANDS:
         gen  Generate Fractal Genesis State Json

    OPTIONS:
       --pass value            The password for keys
       --gstake value          The total stake in genesis state (default: 100000000000000000)
       --packerKeyOwner value  The owner address of packer key contract stake
       --help, -h              show help

gtool keys
------------------------------------
Help
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
.. parsed-literal::
    NAME:
       gtool keys - Manage Fractal Keys

    USAGE:
       gtool keys [command options] subcommand

    SUBCOMMANDS:
         list          List Fractal Keys
         newkeys       New Keys for mining/packer/account
         newminingkey  New Mining Key
         regminingkey  Register Mining Key
         newpackerkey  New Packer Key

    OPTIONS:
       --keys value     The Folder for all the key files
       --pass value     The password for keys
       --addr value     The address for keys
       --rpc value      rpc service address
       --chainid value  chain id (default: 0)
       --help, -h       show help

gtool packer
------------------------------------
Help
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
.. parsed-literal::
    NAME:
       gtool packer - Manage Fractal Packer

    USAGE:
       gtool packer [command options] subcommand

    SUBCOMMANDS:
         start      Start pack service
         stop       Stop pack service
         setPacker  Call Contract

    OPTIONS:
       --rpc value             rpc service address
       --packerId value        packer index (default: 0)
       --chainid value         chain id (default: 0)
       --keys value            The Folder for all the key files
       --pass value            The password for keys
       --abi value             abi file path
       --packerAddress value   packer rpc address
       --packerCoinbase value  packer coinbase
       --packerPubKey value    packer public key (ECDSA)
       --help, -h              show help

gtool state
------------------------------------
Help
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
.. parsed-literal::
    NAME:
       gtool state - Query Fractal State

    USAGE:
       gtool state [command options] subcommand

    SUBCOMMANDS:
         account  Query account info
         storage  Query storage info

    OPTIONS:
       --rpc value    rpc service address
       --addr value   The address for keys
       --table value  table name
       --skey value   storage key
       --help, -h     show help

gtool tx
------------------------------------
Help
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
.. parsed-literal::
    NAME:
       gtool tx - Generate Transaction

    USAGE:
       gtool tx [command options] subcommand

    SUBCOMMANDS:
         send    Send Transaction
         batch   Batch Send Transaction
         deploy  Deploy Contract
         call    Call Contract

    OPTIONS:
       --rpc value       rpc service address
       --packer          whether rpc server is packer or not
       --to value        to address
       --value value     transfer value (default: 1)
       --tps value       tps for current test (default: 0)
       --nprocess value  process count (default: 0)
       --chainid value   chain id (default: 0)
       --keys value      The Folder for all the key files
       --pass value      The password for keys
       --wasm value      wasm file path
       --abi value       abi file path
       --action value    action name
       --args value      args json
       --help, -h        show help

Examples
------------------------------------

Query Enode Address
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
Assume:
    * RPC server address is *http://127.0.0.1:8545*

.. code-block:: bash

    gtool admin --rpc http://127.0.0.1:8545 enode

Query Block with Hash
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
Assume:
    * RPC server address is *http://127.0.0.1:8545*
    * Block hash is *0x2f06e35a6d3b6ef2d9f4abb607082c240ca77e3be9dd23080ee2fc4467411a6f*

.. code-block:: bash

    gtool block --rpc http://127.0.0.1:8545 --hash 0x2f06e35a6d3b6ef2d9f4abb607082c240ca77e3be9dd23080ee2fc4467411a6f query

Query Keys in Local
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
Assume:
    * Your key's password is *888*
    * Your key's folder is *data/keys*

.. code-block:: bash

    gtool keys --keys data/keys --pass 888 list

Query Account Balance
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
Assume:
    * RPC server address is *http://127.0.0.1:8545*
    * Your account address is *0xfd4b1e33d9155b469b87a9a1059d15fdcb67f898*

.. code-block:: bash

    gtool state --rpc http://127.0.0.1:8545 --addr 0xfd4b1e33d9155b469b87a9a1059d15fdcb67f898 account

Send Transaction to Transfer Token
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
Assume:
    * Your key's password is *888*
    * Your key's folder is *data/keys*
    * RPC server address is *http://127.0.0.1:8545*
    * Account address which you want to transfer to, is *0xfd4b1e33d9155b469b87a9a1059d15fdcb67f898*
    * The token amount you want to transfer, is *123456789*
    * You test on Fractal Testnet(chainid: 2)

.. code-block:: bash

    gtool tx --rpc http://127.0.0.1:8545 --keys data/keys --pass 888 --to 0xfd4b1e33d9155b469b87a9a1059d15fdcb67f898 --value 123456789 --chainid 2 send
