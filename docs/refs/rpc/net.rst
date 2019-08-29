net
---

listening
'''''''''

Listening returns an indication if the node is listening for network connections.

Parameters:
"""""""""""
none


Returns:
""""""""
1. If the node us listening for the network connections.


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
               "method": "net_listening",
               "params": []
   }

Responses:

Status: listening | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": true
   }

peerCount
'''''''''

RemovePeer disconnects from a remote node if the connection exists


Parameters:
"""""""""""
none

Returns:
""""""""
1. The count of the peers;


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
               "method": "net_peerCount",
               "params": []
   }

Responses:

Status: peerCount UNWORK | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": "0x0"
   }

peers
'''''

Peers retrieves all the information we know about each individual peer at the
protocol granularity.


Parameters:
"""""""""""
none


Returns:
""""""""
1. The information of all peers;


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
               "method": "net_peers",
               "params": []
   }

Responses:

Status: peers | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": []
   }

nodeInfo
''''''''

NodeInfo retrieves all the information we know about the host node at the
protocol granularity.


Parameters:
"""""""""""
none


Returns:
""""""""
1. The information of the node;


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
               "method": "net_nodeInfo",
               "params": []
   }

Responses:

Status: nodeInfo | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": {
           "id": "f80ab20fc57d15112c650584adfafdca48aa3a30cca0f9bcb4ffdf8638b3b8ea1e1dae6375ff4cadbb0cfea396fd78ffe27a857dad9215eaade69b15be09507f",
           "name": "gftl/v0.2.0-stable/darwin-amd64/go1.12.5",
           "enode": "enode://f80ab20fc57d15112c650584adfafdca48aa3a30cca0f9bcb4ffdf8638b3b8ea1e1dae6375ff4cadbb0cfea396fd78ffe27a857dad9215eaade69b15be09507f@210.22.171.162:30303",
           "ip": "210.22.171.162",
           "ports": {
               "discovery": 30303,
               "listener": 30303
           },
           "listenAddr": "[::]:30303",
           "protocols": {
               "ftl": {
                   "network": 999,
                   "Height": 8666,
                   "genesis": "0xe8c244a7ca2e2470898699590240bd27d785c67c6ed6657be754939171a53fc8",
                   "head": "0x044018d7b28bf31b2145d2a2c2ec5c79e57acc7c26d5e418509a0e928439beb0"
               }
           }
       }
   }

version
'''''''

Version returns the current Fractal protocol version.

Parameters:
"""""""""""
none

Returns:
""""""""
1. The version of the Fractal protocol;


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
               "method": "net_version",
               "params": []
   }

Responses:

Status: nodeInfo | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": "2"
   }
