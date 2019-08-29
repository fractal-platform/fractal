packer
------

getTxPackageByHash
''''''''''''''''''

GetTxPackageByHash returns transaction package by the given hash.

Parameters:
"""""""""""
1. The hash of the transaction package;


Returns:
""""""""
1. The transaction package;


Example:
""""""""

Endpoint:

.. code-block:: bash

   Method: GET
   Type: RAW
   URL: http://{{host}}:8545/rpc

Body:

.. code-block:: js

   {
               "jsonrpc": "2.0",
               "id": "1",
               "method": "pack_getTxPackageByHash",
               "params": ["0xfda689596a9a0f1e184b972b3ca2601f03c0d52abf1db7604356d1eec22f54f4"]
   }


.. UNTESTED

sendRawTransaction
''''''''''''''''''''''''

SendRawTransaction allows to send encoded transaction to the packer.


Parameters:
"""""""""""
1. The encoded transaction;


Returns:
""""""""
1. The hash of the transaction;


Example:
""""""""

Endpoint:

.. code-block:: bash

   Method: GET
   Type: RAW
   URL: http://{{host}}:8545/rpc

Body:

.. code-block:: js

   {
               "jsonrpc": "2.0",
               "id": "1",
               "method": "pack_getTxPackageByHash",
               "params": ["0xfda689596a9a0f1e184b972b3ca2601f03c0d52abf1db7604356d1eec22f54f4"]
   }
