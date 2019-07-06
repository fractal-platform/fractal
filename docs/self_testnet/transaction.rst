Transaction 
------------
includes ``one`` step:

1. send transaction :ref:`detail <send-transaction-label>`.


.. _send-transaction-label:
send transaction
^^^^^^^^^^^^^^^^^^
You can send transactions , we only put ``transfer balance from A user to B user`` here, but for smart contract use , go `smart contract <xxx>`_.
send transaction command is :
:: 
    $  gtool tx --rpc http://127.0.0.1:8545 --to 0xc402b930dbe2a2fec29dc4699dc0c17f19805949  --chainid 999 --keys data/keys --pass 666 send
    t=2019-07-02T19:35:12+0800 lvl=info msg="get nonce ok" nonce=0
    t=2019-07-02T19:35:12+0800 lvl=info msg="send tx success" hash=0x823e7dde4a4a68fad223beaf47124deeec0534a81a838add639b2a9374ed3ca4
    t=2019-07-02T19:35:14+0800 lvl=info msg="recv tx rsp" from=0xDc19ab8A51Ac78eb99392262e26681d64ba66317 nonce=0 hash=0x823e7dde4a4a68fad223beaf47124deeec0534a81a838add639b2a9374ed3ca4 to=0xC402B930dBe2a2FEc29dC4699DC0C17F19805949 receipt=<nil>

**WARNING** ``rpc`` is the chain server, ``to`` is the balance receiver, you must assign ``chainid`` here according to your ``test.toml``, ``chainid`` is the flag
to distinguish testnet environment from main-net environment. ``keys`` is your key directory , ``pass`` is your password.
Transaction amount is fixed to 1 ``ftl``,so you don't need to assign it .


