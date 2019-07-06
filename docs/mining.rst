Mining with gftl
-------------------------

**NOTE:** Ensure your blockchain is fully synchronised with the main chain before starting to mine, otherwise you will not be mining on the main chain.

When you start up your fractal node with ``gftl`` it is not mining by default. To start it in mining mode, you use the ``--mine`` [command line option]. 

``gftl --identity 2.3 --unlock 123 --config test.toml --datadir data --mine``

You can also start and stop CPU mining at runtime using the [console]. 

.. code-block:: shell

    > miner.start()
    true
    > miner.stop()
    true


Note that mining for real fractal only makes sense if you are in sync with the network (since you mine on top of the consensus block).
Therefore the fractal blockchain downloader/synchroniser will delay mining until syncing is complete, and after that mining automatically starts unless you cancel your intention with ``miner.stop()``.

In order to earn token you must have your ``coinbase`` address set. This ``coinbase`` defaults to your [primary account].
If you don't have a ``coinbase`` address, then ``gftl --mine`` will not start up.

You can set your ``coinbase`` through ``--datadir`` on the command line:

::

    gftl --identity 2.3 --config test.toml  --datadir data  --mine

You can reset your ``coinbase`` on the console too:
::

    miner.setCoinBase(ftl.accounts[1])


Note that your ``coinbase`` does not need to be an address of a local account, just an existing one. 


In order to spend your earnings you will need to have this account unlocked.

.. code-block:: shell

    > personal.unlockAccount(ftl.coinbase)
    Password
    true
    

Note that it will happen often that you find a block yet it never makes it to the canonical chain. 
This means when you locally include your mined block, the current state will show the mining reward credited to your account,
 however, after a while, the better chain is discovered and we switch to a chain in which your block is not included and 
 therefore no mining reward is credited. Therefore it is quite possible that as a miner monitoring their coinbase balance will find that it may fluctuate quite a bit. 

The logs show locally mined blocks confirmed after 5 blocks. At the moment you may find it easier and faster to generate the list of your mined blocks from these logs.

Mining success depends on the set block difficulty and your balance. 
Your chances of finding a block therefore follows from chain difficulty and your balance. The time you need to wait you are expected to find a block can be estimated with the following code:

::

    ftm = miner.getDifficulty()/stake; // estimated time in seconds

Given a difficulty of 3 billion, miner stake is 1.5 billion,the miner is expected to find a block every 2 seconds.


