admin
-----

addPeer
'''''''

AddPeer requests connecting to a remote node, and also maintaining the new
connection at all times, even reconnecting if it is lost.


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
               "method": "admin_addPeer",
               "params": ["enode://9fb5d28a4f0d086521bb3824782ad8b3d982190afc839200276e926dd2661b804854e609cd95a7bbc8969f2d513aac6a74c7c839409e786697d01ceb50fb2919@210.22.171.162:30304"]
   }

Responses:

Status: addPeer | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": true
   }

removePeer
''''''''''

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
               "method": "admin_removePeer",
               "params": ["enode://9fb5d28a4f0d086521bb3824782ad8b3d982190afc839200276e926dd2661b804854e609cd95a7bbc8969f2d513aac6a74c7c839409e786697d01ceb50fb2919@210.22.171.162:30304"]
   }

Responses:

Status: removePeer | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": true
   }

addTrustedPeer
''''''''''''''

AddTrustedPeer allows a remote node to always connect, even if slots are full


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
               "method": "admin_addTrustedPeer",
               "params": ["enode://9fb5d28a4f0d086521bb3824782ad8b3d982190afc839200276e926dd2661b804854e609cd95a7bbc8969f2d513aac6a74c7c839409e786697d01ceb50fb2919@210.22.171.162:30304"]
   }

Responses:

Status: addTrustedPeer | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": true
   }

removeTrustedPeer
'''''''''''''''''

RemoveTrustedPeer removes a remote node from the trusted peer set, but it 
does not disconnect it automatically.



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
               "method": "admin_removeTrustedPeer",
               "params": ["enode://9fb5d28a4f0d086521bb3824782ad8b3d982190afc839200276e926dd2661b804854e609cd95a7bbc8969f2d513aac6a74c7c839409e786697d01ceb50fb2919@210.22.171.162:30304"]
   }

Responses:

Status: removeTrustedPeer | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": true
   }

addBlack
''''''''

Addblack rejects the connection of the specified peer node.

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
               "method": "admin_addBlack",
               "params": ["enode://9fb5d28a4f0d086521bb3824782ad8b3d982190afc839200276e926dd2661b804854e609cd95a7bbc8969f2d513aac6a74c7c839409e786697d01ceb50fb2919@210.22.171.162:30304"]
   }

Responses:

Status: addBlack | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": true
   }

removeBlack
'''''''''''

RemoveBlack removes a peer node from the lacklist.


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
               "method": "admin_removeBlack",
               "params": ["enode://9fb5d28a4f0d086521bb3824782ad8b3d982190afc839200276e926dd2661b804854e609cd95a7bbc8969f2d513aac6a74c7c839409e786697d01ceb50fb2919@210.22.171.162:30304"]
   }

stopMining
''''''''''

StopMining stops the node from mining new blocks.


Parameters:
"""""""""""
none

Returns:
""""""""
1. The err of the request, null if success;


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
               "method": "admin_stopMining",
               "params": ["enode://9fb5d28a4f0d086521bb3824782ad8b3d982190afc839200276e926dd2661b804854e609cd95a7bbc8969f2d513aac6a74c7c839409e786697d01ceb50fb2919@210.22.171.162:30304"]
   }

Responses:

Status: stopMining | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": null
   }

startMining
'''''''''''

StartMining starts the mining of the node.

Parameters:
"""""""""""
none

Returns:
""""""""
1. The err of the request, null if success;


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
               "method": "admin_startMining",
               "params": []
   }

Responses:

Status: startMining | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": null
   }

Mining
''''''

Mining returns an indication if this node is currently mining.

Parameters:
"""""""""""
none

Returns:
""""""""
1. If the node is currently mining, true/false;


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
               "method": "admin_mining",
               "params": []
   }

Responses:

Status: Mining | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": true
   }

generateMiningKey
'''''''''''''''''

GenerateMiningKey generates mining key with a given address.

Parameters:
"""""""""""
1. The address of the miner;


Returns:
""""""""
1. The Public key of the mining;


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
               "method": "admin_generateMiningKey",
               "params": ["0xa04358d378cf97a933eb09b6014f4f118378e9f4"]
   }

Responses:

Status: generateMiningKey | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": [
           87,
           ...,
           156
       ]
   }

startPacking
''''''''''''

StartMining starts the packing of the node.


Parameters:
"""""""""""
none


Returns:
""""""""
1. the error of the request, null if success;


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
               "method": "admin_startPacking",
               "params": [1]
   }

Responses:

Status: startPacking | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": null
   }

isPacking
'''''''''

isPacking returns an indication if this node is currently packing.

Parameters:
"""""""""""
none

Returns:
""""""""
1. If the node is currently packing, true/false;


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
               "method": "admin_isPacking",
               "params": []
   }

Responses:

Status: isPacking | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": true
   }

.. toctree::
  :maxdepth: 1
