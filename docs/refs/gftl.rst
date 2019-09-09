Command Line Usage <gftl>
====================================
Help
------------------------------------
.. parsed-literal::
    gftl [options]

    VERSION:
       0.2.0-stable

    OPTIONS:
       --config value            TOML configuration file
       --genesisAlloc value      genesis_alloc configuration file
       --checkPoint value        checkPoints configuration file
       --datadir "data"          Data directory for the databases and keys
       --testnet                 Test network: pre-configured test network
       --testnet2                Test network: pre-configured test2 network
       --testnet3                Test network: pre-configured test3 network
       --synctest                test fastsync pre-configured test fastsync
       --mine                    Enable mining
       --packer                  Enable packer
       --packerId value          Set packer index (default: 0)
       --unlock value            The password to use for unlock the miner's private key
       --rpc                     Enable the HTTP-RPC server
       --rpcaddr value           HTTP-RPC server listening interface (default: "localhost")
       --rpcport value           HTTP-RPC server listening port (default: 8545)
       --rpcapi value            HTTP-RPC server api list
       --rpccorsdomain value     Comma separated list of domains from which to accept cross origin requests (browser enforced)
       --identity value          Custom node name
       --maxpeers value          Maximum number of network peers (network disabled if set to 0) (default: 25)
       --maxpendpeers value      Maximum number of pending connection attempts (defaults used if set to 0) (default: 0)
       --port value              Network listening port (default: 30303)
       --bootnodes value         Comma separated enode URLs for P2P discovery bootstrap
       --nat value               NAT port mapping mechanism (any|none|upnp|pmp|extip:<IP>) (default: "none")
       --nodiscover              Disables the peer discovery mechanism (manual peer addition)
       --metrics                 Enable metrics collection and reporting
       --influxdburl value       Influxdb url for metrics
       --influxdbdatabase value  Influxdb database for metrics
       --influxdbusername value  Influxdb username for metrics
       --influxdbpassword value  Influxdb password for metrics
       --verbosity value         Logging verbosity: 0=silent, 1=error, 2=warn, 3=info, 4=debug (default: 3)
       --pprof                   Enable the pprof HTTP server
       --pprofport value         pprof HTTP server listening port (default: 6060)
       --pprofaddr value         pprof HTTP server listening interface (default: "127.0.0.1")
       --help, -h                show help

Chain Options
------------------------------------
--datadir data
    the folder stores chaindata and keys

--testnet
    connect to testnet

--testnet2
    connect to testnet2

--testnet3
    connect to testnet3

.. hint:: For most people, choose testnet for your node.

--config value
    TOML configuration file

--genesisAlloc value
    genesis alloc configuration file

.. hint:: Options config/genesisAlloc are only used for private network deploy.

Mine Options
------------------------------------
--mine
    enable mining

.. hint:: You must register your mining keys before you start mining, visit `here <../guides/index.html#deploy-miner-node>`_ for more information.

--unlock value
    the password to use for unlock the miner's private key

RPC Options
------------------------------------
--rpc
    Enable the HTTP-RPC server

--rpcaddr value
    HTTP-RPC server listening interface (default: "localhost")

.. hint:: If you want to serve rpc request from other PC, you can set rpcaddr to 0.0.0.0 or your internet ip address.

--rpcport value
    HTTP-RPC server listening port (default: 8545)

--rpcapi value
    HTTP-RPC server api list

--rpccorsdomain value
    Comma separated list of domains from which to accept cross origin requests (browser enforced)

Network Options
------------------------------------
--port value
    Network listening port (default: 30303)

.. hint:: It is the communication port between your node and other peers.

--bootnodes value
    Comma separated enode URLs for P2P discovery bootstrap

.. hint:: Options config/genesisAlloc are only used for private network deploy.

--nat value
    NAT port mapping mechanism (any|none|upnp|pmp|extip:<IP>) (default: "none")

.. hint:: If you are behind a router, you should use this option.

Examples
------------------------------------
Start a node without mining on Fractal Testnet
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
Assume:
    * Your data folder is *data*
    * Your key's password is *888*

.. code-block:: console

    $ gftl --testnet --rpc --datadir data --unlock 888

Start a node with mining on Fractal Testnet
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
Assume:
    * Your data folder is *data*
    * Your key's password is *888*
    * You have registered your mining keys

.. code-block:: console

    $ gftl --testnet --rpc --datadir data --unlock 888 --mine

Start a node with distinct port on Fractal Testnet
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
Assume:
    * Your data folder is *data*
    * Your key's password is *888*
    * You want to use port 36666 for data transfer

.. code-block:: console

    $ gftl --testnet --port 36666 --rpc --datadir data --unlock 888

Start a node with distinct rpc address/port on Fractal Testnet
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
Assume:
    * Your data folder is *data*
    * Your key's password is *888*
    * You want to use 0.0.0.0:8080 for rpc service

.. code-block:: console

    $ gftl --testnet --rpc --rpcaddr 0.0.0.0 --rpcport 8080 --datadir data --unlock 888


