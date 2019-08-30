txpool
------

getTransactionByHash
'''''''''''''''''''''''''

GetTransactionByHash returns a Transaction by the given hash.

Parameters:
"""""""""""
1. The hash of the transaction;


Returns:
""""""""
1. The transaction;


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
               "method": "txpool_getTransactionByHash",
               "params": ["0xfda689596a9a0f1e184b972b3ca2601f03c0d52abf1db7604356d1eec22f54f4"]
   }

Responses:

Status: content | Code: 200

.. code-block:: js

   {
      "jsonrpc": "2.0",
      "id": "1",
      "result": {
         "from": "0xf13376df4b6e043d0e3e6d6561272875aa287294",
         "hash": "0x1d87e584da5f8f8fa26e581ff8f24972f2d1ec82bfd23bc1c2216d69337e5ae0",
         "nonce": "0x127e00b",
         "to": "0x3250a7bba25e342e70d5dbaf1e66649166789df2",
         "value": "0x1",
         "v": "0x28",
         "r": "0x8e159f4972804e1fffb15284d75568f33f91636c1e8bd916579a244b33acb74c",
         "s": "0x3744b594344d6a47e24a7bce08b3e0459c199dc94a375b4c3f05f93b03fe9778",
         "blockHash": "0x8f1cd787fb1f3efda184dc707eee43861c8004a3bcb3fce6651bdb6a6a9fe3b2",
         "receipt": {
               "root": "0x",
               "status": "0x1",
               "cumulativeGasUsed": "0x42e05c80",
               "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
               "logs": [],
               "transactionHash": "0x1d87e584da5f8f8fa26e581ff8f24972f2d1ec82bfd23bc1c2216d69337e5ae0",
               "contractAddress": "0x0000000000000000000000000000000000000000",
               "gasUsed": "0x1e8480"
         }
      }
   }

content
'''''''

Content returns the transactions contained within the transaction pool.


Parameters:
"""""""""""
none


Returns:
""""""""
1. The transactions;


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
               "method": "txpool_content",
               "params": []
   }

Responses:

Status: content | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": {
           "queued": {}
       }
   }

status
''''''

Status returns the number of pending and queued transactions in the pool.



Parameters:
"""""""""""
none


Returns:
""""""""
1. The number of the transactions;


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
               "method": "txpool_status",
               "params": []
   }

Responses:

Status: status | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": {
           "queued": "0x0"
       }
   }

inspect
'''''''

Inspect retrieves the content of the transaction pool and flattens it into an
easily inspectable list.



Parameters:
"""""""""""
none


Returns:
""""""""
1. The list of content in the pool;


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
               "method": "txpool_inspect",
               "params": []
   }

Responses:

Status: inspect | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": {
           "queued": {}
       }
   }

getTransactionNonce
'''''''''''''''''''

GetTransactionNonce returns the transaction nonce of an address.


Parameters:
"""""""""""
1. The hash of the address;


Returns:
""""""""
1. The nonce of the transactions;


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
               "method": "txpool_getTransactionNonce",
               "params": ["0xf13376df4b6e043d0e3e6d6561272875aa287294"]
   }

Responses:

Status: getTransactionNonce | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": "0x0"
   }

getBlockTransactionCountByHash
''''''''''''''''''''''''''''''

GetBlockTransactionCountByHash returns the number of transactions in the block with the given hash.


Parameters:
"""""""""""
1. The hash of the block;


Returns:
""""""""
1. The number of the transactions;


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
               "method": "txpool_getBlockTransactionCountByHash",
               "params": ["0xd1c0f4f8e1ef3fb27bc19a3d0641f2e28cc61689340b10f73b3fdc65d0955fdf"]
   }

Responses:

Status: getBlockTransactionCountByHash | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": "0x0"
   }

Status: getBlockTransactionCountByHash | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": "0x0"
   }

.. UNTESTED

sendRawTransaction
'''''''''''''''''''''''

Creates new message call transaction or a contract creation for signed transactions.


Parameters:
"""""""""""
1. The signed transaction data;


Returns:
""""""""
1. The hash of the transaction or zero if unavailable;


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
               "method": "txpool_SendRawTransaction",
               "params": ["0xd1c0f4f8e1ef3fb27bc19a3d0641f2e28cc61689340b10f73b3fdc65d0955fdf"]
   }

Responses:

Status: encodeAction | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": "0x1d87e584da5f8f8fa26e581ff8f24972f2d1ec82bfd23bc1c2216d69337e5ae0"
   }


.. UNTESTED

encodeAction
'''''''''''''''''

RemovePeer disconnects from a remote node if the connection exists


Parameters:
"""""""""""
1. The url of the new peer node;


Returns:
""""""""
1. The result of the request, true/false;


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
               "method": "txpool_encodeAction",
               "params": ["0xd1c0f4f8e1ef3fb27bc19a3d0641f2e28cc61689340b10f73b3fdc65d0955fdf", "sdada", "sadasd"]
   }

Responses:

Status: encodeAction | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": "0x"
   }

.. UNTESTED

call
''''''''''''''''''''''''

PendingTransactions returns all the pending transactions in the pool.


Parameters:
"""""""""""
1. The arguments of sending transactions:
- from(optional): The address the transaction is sent from.
- to: The address the transaction is directed to.
- gas(optional): Integer of the gas provided for the transaction execution. This Call method consumes zero gas, but this parameter may be needed by some executions.
- gasPrice(optional): Integer of the gasPrice used for each paid gas
- value(optional): Integer of the value sent with this transaction
- nonce(optional): Sequence of the transaction
- data(optional): Hash of the method signature and encoded parameters.


Returns:
""""""""
1. The return value of the executed contract;


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
               "method": "txpool_call",
               "params": [{{send_transaction_args}}]
   }

Responses:

Status: pendingTransactions | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": "0x"
   }


.. UNTESTED

pendingTransactions
''''''''''''''''''''''''

PendingTransactions returns all the pending transactions in the pool.


Parameters:
"""""""""""
none


Returns:
""""""""
1. The transactions;


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
               "method": "txpool_pendingTransactions",
               "params": []
   }

Responses:

Status: pendingTransactions | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": null
   }

gasPrice
''''''''

GasPrice returns the gas price of pool.


Parameters:
"""""""""""
nonce


Returns:
""""""""
1. The gas price;


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
               "method": "txpool_gasPrice",
               "params": []
   }

Responses:

Status: gasPrice | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": "0x1"
   }

.. toctree::
  :maxdepth: 1
