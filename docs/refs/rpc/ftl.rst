ftl
---

headBlock
''''''''''''''''''

HeadBlock returns the head block in the chain.

Parameters:
"""""""""""
none


Returns:
""""""""
1. The head block of the chain


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
               "method": "ftl_headBlock",
               "params": []
   }

Responses:

Status: headBlock | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": {
           "Body": {
               "transactions": null,
               "txpackages": null
           },
           "FullHash": "0x38b4cf33a7651a6994791e221acb947c652b49bd821bd0a42a0b2b6358a713f1",
           "Header": {
               "parentHash": "0x01a7a11a563f8960553019398317687ff98c6bbac11d2c804fff68c4426ad823",
               "round": 15664543878,
               "sig": "eynNPuGppIV8FSn/XpnrifxwFwFKtCYJsoE2wvjFV4dcp/+QTYv+o6bHjUl8irAEhphcBqf+teUin/TnEcataw==",
               "miner": "0xa04358d378cf97a933eb09b6014f4f118378e9f4",
               "difficulty": 169686360399196945,
               "height": 8622,
               "amount": 43783,
               "gasLimit": 2024778145554138,
               "gasUsed": 0,
               "stateHash": "0x274efd8ee3369511f53518d95990c7800ee0f569c4e7cb1fe99c3b8c25bf11d1",
               "txHash": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
               "receiptsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
               "parentFullHash": "0x60162cb9852d533288bed466d94eac6c8320e5665bdbf91d71041543172a7c00",
               "confirms": [
                   "0x6dce777a3533dc18974cfe7469da27ad29cebd8b6ad89469fd40933921c9a21b",
                   "0x26f91d8f52f3fc6b0bee8f044bfa4235efd6c7e45dd8d7cd60a380801a08951c"
               ],
               "fullSig": "Ry0qj9MHDJOedncR+LyPdHQTuYHV081C2t7CGzUqyot2DFEHqeXLOiZ2Fj09FXythdC6oOLE2tFegyvnBDIz1A==",
               "minedTime": 1566454387810,
               "hopCount": 0
           },
           "SimpleHash": "0x0159a0a80bf4d6b1521d9bdb03b9f6e973a0b58deb01f6566d9d3b12f8bbe13f"
       }
   }

genesis
'''''''

Genesis returns the first block in the chain.


Parameters:
"""""""""""
none


Returns:
""""""""
1. The genesis block of the chain.


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
               "method": "ftl_genesis",
               "params": []
   }

Responses:

Status: genesis | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": {
           "Body": {
               "transactions": [],
               "txpackages": []
           },
           "FullHash": "0xe8c244a7ca2e2470898699590240bd27d785c67c6ed6657be754939171a53fc8",
           "Header": {
               "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
               "round": 15662888829,
               "sig": "AA==",
               "miner": "0x0000000000000000000000000000000000000000",
               "difficulty": 100000000000000000,
               "height": 0,
               "amount": 1,
               "gasLimit": 9223372036854775807,
               "gasUsed": 0,
               "stateHash": "0x2209bca809b40e32c5f6770a987b65cb823cb88838bff7e42a9927d272a4e2e6",
               "txHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
               "receiptsRoot": "0x0000000000000000000000000000000000000000000000000000000000000000",
               "parentFullHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
               "confirms": [],
               "fullSig": "",
               "minedTime": 0,
               "hopCount": 0
           },
           "SimpleHash": "0x9a9ae16dbd917f01daccd9c40a0e11732c7a15b3bfd1d5f932dbfaafe9f17d09"
       }
   }

getBlock
''''''''

GetBlock returns the block with the hash.


Parameters:
"""""""""""
1. The hash of the block;


Returns:
""""""""
1. The block with the hash;


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
               "method": "ftl_getBlock",
               "params": ["0xd1c0f4f8e1ef3fb27bc19a3d0641f2e28cc61689340b10f73b3fdc65d0955fdf"]
   }

Responses:

Status: getBlock | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": {
           "Body": {
               "transactions": [],
               "txpackages": []
           },
           "FullHash": "0xd1c0f4f8e1ef3fb27bc19a3d0641f2e28cc61689340b10f73b3fdc65d0955fdf",
           "Header": {
               "parentHash": "0x014b56ae547a4c346f21e199417689362bc9aabdd2f43baf81bab9f08c537420",
               "round": 15663843083,
               "sig": "FX36HHWvKvrj81O4rdWZeItM9wbOhMlDOK1/SHuw/GBkGJ25qfGezHqq1mQKd14dkYuvgNh8xlbkRmeDP10dBA==",
               "miner": "0xa04358d378cf97a933eb09b6014f4f118378e9f4",
               "difficulty": 125540539788043314,
               "height": 943,
               "amount": 4842,
               "gasLimit": 3670736464583360781,
               "gasUsed": 0,
               "stateHash": "0x5efa810863ac3dbb790d70fbd4f564bafc21b0d1b5e8bdfc256f6cbd871fbdc0",
               "txHash": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
               "receiptsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
               "parentFullHash": "0xb5815effd12d4170401359ebc28e06f1dccc9655ecf8a1c2b6ef2ea1fe7ee5f0",
               "confirms": [
                   "0xce39c8a5fd8d224898aa19951505b3c657f9259650e0cfa45c5d13db282895d7",
                   "0x0aa8ddd7a5268b3b4636a4c0c18d4fbb99eb231c2362a3f8e561795501e3aaa8",
                   "0x7c0589ec94a1ad887a63cfbad225097f2d86db7f6628368c9c4b27714a7b86c5",
                   "0x09a55f4c0cfaa1dd3bb3fb70c920d150157f3e8200855cbeceb3d6cd3e663101",
                   "0x4c66dde446f65b46d703cd96337bfe42203e8e46925c04980f2ce3ced0455d11",
                   "0xe308587bbe23eec2ae9ddde7f52a727036c60e4fc8249493f95f1248c555cf8f",
                   "0x9056413bddd7c5dfc6dfd5eb458752cae1be4317277e4ce95041fe7eb9037876"
               ],
               "fullSig": "AUEgR1dn6GMmcFLjA7vke74+r/vdPspeL6AvpGbAlkMLSPpuZk3GC2IyjPX4pbtQWBfNwDZHYB/RKDcuu22ZeQ==",
               "minedTime": 1566384308308,
               "hopCount": 0
           },
           "SimpleHash": "0x01f7fe195b39a92b6ae42f74720066f7e67f72f37905990f47e263e4eaa70dfa"
       }
   }

getBlockByHeight
''''''''''''''''''

GetBlockHeight returns the main block on given height of the chain.


Parameters:
"""""""""""
1. The height;


Returns:
""""""""
1. The main block on the given height;


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
               "method": "ftl_getBlockByHeight",
               "params": ["0x2"]
   }

Responses:

Status: getBlockByHeight | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": {
           "Body": {
               "transactions": [],
               "txpackages": []
           },
           "FullHash": "0x4fbb1762f9d5984cc4010de0fa07bae2e2953c22856e5b7b08c1dcf667fbf602",
           "Header": {
               "parentHash": "0x00941c3b9c5a2e776fa53a1c19beafb5ecb86b97464602b758006b618eaf4908",
               "round": 15663776923,
               "sig": "Pr/ZozFJp+UwyKBMoPhQsnROPJB7wYOqn9uQzIEYNHYM7AL7YE23CKRE+Tlc9FtuGIfTsEOs6arUzpM6XmTdzw==",
               "miner": "0xa04358d378cf97a933eb09b6014f4f118378e9f4",
               "difficulty": 95212483406066894,
               "height": 2,
               "amount": 4,
               "gasLimit": 9205366434438316034,
               "gasUsed": 0,
               "stateHash": "0xe13bef9148a50fced37e35509774258c3fe621ca4e4ddd0bc439d5eb2f4481bb",
               "txHash": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
               "receiptsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
               "parentFullHash": "0xc843e9611fddcf42fcf1a35df5bb7cadf56fa89cef6edea74f5336b63d79cb16",
               "confirms": [
                   "0x9c7cd4f44da3e88764b999fe7f2cd14810df28f55a2fc7af6cd845adbe2899b4"
               ],
               "fullSig": "a/3hzUJ/Q4zUpnZNnwgaL9pn5BqHXmgSr4W/L3LyGnQwOWO/ueFHJu27gP4wQ20UMgb/VAQIIj5jiOc4ycZb2g==",
               "minedTime": 1566377692306,
               "hopCount": 0
           },
           "SimpleHash": "0x012a9e8e077cef1968faca4c93b8421f20e810ec988746eca86e55dcbb3f5e93"
       }
   }

blockHeight
'''''''''''

BlockHeight returns the block heigth of the chain head.


Parameters:
"""""""""""
none


Returns:
""""""""
1. The height of the chain;


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
               "method": "ftl_blockHeight",
               "params": []
   }

Responses:

Status: blockHeight | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": "0x21b5"
   }

getBackwardBlocks
'''''''''''''''''

GetBackwardBlocks return the given amount of blocks of which are pre-produced of the given block.

Parameters:
"""""""""""
1. The hash of the block;
2. The amount of the blocks;


Returns:
""""""""
1. The pre-produced blocks;


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
               "method": "ftl_getBackwardBlocks",
               "params": ["0xd1c0f4f8e1ef3fb27bc19a3d0641f2e28cc61689340b10f73b3fdc65d0955fdf", 2]
   }

Responses:

Status: getBackwardBlocks | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": [
           {
               "Body": {
                   "transactions": [],
                   "txpackages": []
               },
               "FullHash": "0xe9fbad10f15fea59a7c39d84415dbf08976cc925d999085daddad415fe0dc9f6",
               "Header": {
                   "parentHash": "0x01111daf044f8459ce6497bce55d3d4bf75b6a2d7767c9b4d810449cc6377dfe",
                   "round": 15663843064,
                   "sig": "cHkNBUVhV/mJ5eoDHd+OO6ERjPZJRIV3GmktjKjZlPlCswypD+5hylzYB6Vt38zDof+CV9Xe5e5gMTGXiyYfHA==",
                   "miner": "0xa04358d378cf97a933eb09b6014f4f118378e9f4",
                   "difficulty": 125417461709949368,
                   "height": 942,
                   "amount": 4834,
                   "gasLimit": 3674324672271125551,
                   "gasUsed": 0,
                   "stateHash": "0x2eec3eca195615223f277900890ca1d1b52ab13f93777f23bbe1b27834cde5cb",
                   "txHash": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
                   "receiptsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
                   "parentFullHash": "0xb25d9e9a0c8c02adc44df206a2b937f043f67225cc30fc87b39d1d161cbdace7",
                   "confirms": [],
                   "fullSig": "UWOWD+6sSermc9Ri4/z4iq0pb9FTaKMWyPwwkUB7vqwBV/+XWIYuHSZKtbArNTiBlzo3w1o0C8v+5qEu/O6ATg==",
                   "minedTime": 1566384306407,
                   "hopCount": 0
               },
               "SimpleHash": "0x01aaf5501b96f096a108360735c2ec2f06776064781edf7bf148e27ea5b52f60"
           },
           {
               "Body": {
                   "transactions": [],
                   "txpackages": []
               },
               "FullHash": "0xda835f65c25d533a2a3e30085becdab70af1e3e604b194d566e516fc74906cff",
               "Header": {
                   "parentHash": "0x01111daf044f8459ce6497bce55d3d4bf75b6a2d7767c9b4d810449cc6377dfe",
                   "round": 15663843065,
                   "sig": "JRRjc5SBbvz3VXnifjZnDoH2bIQbYAH68kIZDhVW2leAkqwoS8FiHU4L+Uc9YFpd2d71nMew9PsHBHiv+ooE9g==",
                   "miner": "0xa04358d378cf97a933eb09b6014f4f118378e9f4",
                   "difficulty": 125417461709949368,
                   "height": 942,
                   "amount": 4834,
                   "gasLimit": 3674324672271125551,
                   "gasUsed": 0,
                   "stateHash": "0x2eec3eca195615223f277900890ca1d1b52ab13f93777f23bbe1b27834cde5cb",
                   "txHash": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
                   "receiptsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
                   "parentFullHash": "0xb25d9e9a0c8c02adc44df206a2b937f043f67225cc30fc87b39d1d161cbdace7",
                   "confirms": [],
                   "fullSig": "EIZgn93E18c77N1MVCU3Rtw8Dmgzmhzs9hJQbadunjNWz6s39dxj4WSwXKU72WIEErR78Pn5OfNQdHwRp7lyqQ==",
                   "minedTime": 1566384306504,
                   "hopCount": 0
               },
               "SimpleHash": "0x01d5f1130f0b3145735d04da824c0b6ce190b361f7bb2d96f73702a13707118b"
           }
       ]
   }

getAncestorBlocks
'''''''''''''''''

GetAncestorBlocks returns the given amount of blocks which are the ancestors of the given block.


Parameters:
"""""""""""
1. The hash of the block;
2. The amount of the blocks;


Returns:
""""""""
1. The ancestor blocks;


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
               "method": "ftl_getAncestorBlocks",
               "params": ["0xd1c0f4f8e1ef3fb27bc19a3d0641f2e28cc61689340b10f73b3fdc65d0955fdf", 2]
   }

Responses:

Status: getAncestorBlocks | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": [
           {
               "Body": {
                   "transactions": [],
                   "txpackages": []
               },
               "FullHash": "0xd1c0f4f8e1ef3fb27bc19a3d0641f2e28cc61689340b10f73b3fdc65d0955fdf",
               "Header": {
                   "parentHash": "0x014b56ae547a4c346f21e199417689362bc9aabdd2f43baf81bab9f08c537420",
                   "round": 15663843083,
                   "sig": "FX36HHWvKvrj81O4rdWZeItM9wbOhMlDOK1/SHuw/GBkGJ25qfGezHqq1mQKd14dkYuvgNh8xlbkRmeDP10dBA==",
                   "miner": "0xa04358d378cf97a933eb09b6014f4f118378e9f4",
                   "difficulty": 125540539788043314,
                   "height": 943,
                   "amount": 4842,
                   "gasLimit": 3670736464583360781,
                   "gasUsed": 0,
                   "stateHash": "0x5efa810863ac3dbb790d70fbd4f564bafc21b0d1b5e8bdfc256f6cbd871fbdc0",
                   "txHash": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
                   "receiptsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
                   "parentFullHash": "0xb5815effd12d4170401359ebc28e06f1dccc9655ecf8a1c2b6ef2ea1fe7ee5f0",
                   "confirms": [
                       "0xce39c8a5fd8d224898aa19951505b3c657f9259650e0cfa45c5d13db282895d7",
                       "0x0aa8ddd7a5268b3b4636a4c0c18d4fbb99eb231c2362a3f8e561795501e3aaa8",
                       "0x7c0589ec94a1ad887a63cfbad225097f2d86db7f6628368c9c4b27714a7b86c5",
                       "0x09a55f4c0cfaa1dd3bb3fb70c920d150157f3e8200855cbeceb3d6cd3e663101",
                       "0x4c66dde446f65b46d703cd96337bfe42203e8e46925c04980f2ce3ced0455d11",
                       "0xe308587bbe23eec2ae9ddde7f52a727036c60e4fc8249493f95f1248c555cf8f",
                       "0x9056413bddd7c5dfc6dfd5eb458752cae1be4317277e4ce95041fe7eb9037876"
                   ],
                   "fullSig": "AUEgR1dn6GMmcFLjA7vke74+r/vdPspeL6AvpGbAlkMLSPpuZk3GC2IyjPX4pbtQWBfNwDZHYB/RKDcuu22ZeQ==",
                   "minedTime": 1566384308308,
                   "hopCount": 0
               },
               "SimpleHash": "0x01f7fe195b39a92b6ae42f74720066f7e67f72f37905990f47e263e4eaa70dfa"
           },
           {
               "Body": {
                   "transactions": [],
                   "txpackages": []
               },
               "FullHash": "0xb5815effd12d4170401359ebc28e06f1dccc9655ecf8a1c2b6ef2ea1fe7ee5f0",
               "Header": {
                   "parentHash": "0x01111daf044f8459ce6497bce55d3d4bf75b6a2d7767c9b4d810449cc6377dfe",
                   "round": 15663842884,
                   "sig": "YvWh9Mh2eEtoBDPo7A3ovOlz6uybKUYk3MD7p49CH1oD1xQIc1/iwDYvmeAD5JFzzl2W5KxDOQywGIslL+N43g==",
                   "miner": "0xa04358d378cf97a933eb09b6014f4f118378e9f4",
                   "difficulty": 125724706839077118,
                   "height": 942,
                   "amount": 4834,
                   "gasLimit": 3674324672271125551,
                   "gasUsed": 0,
                   "stateHash": "0x2eec3eca195615223f277900890ca1d1b52ab13f93777f23bbe1b27834cde5cb",
                   "txHash": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
                   "receiptsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
                   "parentFullHash": "0xb25d9e9a0c8c02adc44df206a2b937f043f67225cc30fc87b39d1d161cbdace7",
                   "confirms": [],
                   "fullSig": "eXZ/yeSRETUyHfESfJAzVIUZ2/21KxSdzMa+ZK1ZDaksphS1ZAh/VhpRoaS10Y4LsoeBLT4NNdxi24sx2EN/nQ==",
                   "minedTime": 1566384288402,
                   "hopCount": 0
               },
               "SimpleHash": "0x014b56ae547a4c346f21e199417689362bc9aabdd2f43baf81bab9f08c537420"
           }
       ]
   }

getDescendantBlocks
'''''''''''''''''''

GetDescendantBlocks returns the given amount of blocks which are the descendant blocks of the given block.


Parameters:
"""""""""""
1. The hash of the block;
2. The amount of the blocks;


Returns:
""""""""
1. The descendant blocks;


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
               "method": "ftl_getDescendantBlocks",
               "params": ["0xd1c0f4f8e1ef3fb27bc19a3d0641f2e28cc61689340b10f73b3fdc65d0955fdf", 4]
   }

Responses:

Status: getDescendantBlocks | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": [
           {
               "Body": {
                   "transactions": [],
                   "txpackages": []
               },
               "FullHash": "0xd1c0f4f8e1ef3fb27bc19a3d0641f2e28cc61689340b10f73b3fdc65d0955fdf",
               "Header": {
                   "parentHash": "0x014b56ae547a4c346f21e199417689362bc9aabdd2f43baf81bab9f08c537420",
                   "round": 15663843083,
                   "sig": "FX36HHWvKvrj81O4rdWZeItM9wbOhMlDOK1/SHuw/GBkGJ25qfGezHqq1mQKd14dkYuvgNh8xlbkRmeDP10dBA==",
                   "miner": "0xa04358d378cf97a933eb09b6014f4f118378e9f4",
                   "difficulty": 125540539788043314,
                   "height": 943,
                   "amount": 4842,
                   "gasLimit": 3670736464583360781,
                   "gasUsed": 0,
                   "stateHash": "0x5efa810863ac3dbb790d70fbd4f564bafc21b0d1b5e8bdfc256f6cbd871fbdc0",
                   "txHash": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
                   "receiptsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
                   "parentFullHash": "0xb5815effd12d4170401359ebc28e06f1dccc9655ecf8a1c2b6ef2ea1fe7ee5f0",
                   "confirms": [
                       "0xce39c8a5fd8d224898aa19951505b3c657f9259650e0cfa45c5d13db282895d7",
                       "0x0aa8ddd7a5268b3b4636a4c0c18d4fbb99eb231c2362a3f8e561795501e3aaa8",
                       "0x7c0589ec94a1ad887a63cfbad225097f2d86db7f6628368c9c4b27714a7b86c5",
                       "0x09a55f4c0cfaa1dd3bb3fb70c920d150157f3e8200855cbeceb3d6cd3e663101",
                       "0x4c66dde446f65b46d703cd96337bfe42203e8e46925c04980f2ce3ced0455d11",
                       "0xe308587bbe23eec2ae9ddde7f52a727036c60e4fc8249493f95f1248c555cf8f",
                       "0x9056413bddd7c5dfc6dfd5eb458752cae1be4317277e4ce95041fe7eb9037876"
                   ],
                   "fullSig": "AUEgR1dn6GMmcFLjA7vke74+r/vdPspeL6AvpGbAlkMLSPpuZk3GC2IyjPX4pbtQWBfNwDZHYB/RKDcuu22ZeQ==",
                   "minedTime": 1566384308308,
                   "hopCount": 0
               },
               "SimpleHash": "0x01f7fe195b39a92b6ae42f74720066f7e67f72f37905990f47e263e4eaa70dfa"
           },
           ...,
       ]
   }

getNearbyBlocks
'''''''''''''''

getNearbyBlocks returns the given amount of blocks which are nearby th given block.


Parameters:
"""""""""""
1. The hash of the block;
2. The amount of the blocks;

Returns:
""""""""
1. The nearby blocks of the given block;


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
               "method": "ftl_getNearbyBlocks",
               "params": ["0xd1c0f4f8e1ef3fb27bc19a3d0641f2e28cc61689340b10f73b3fdc65d0955fdf", 4]
   }

Responses:

Status: getNearbyBlocks | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": [
           {
               "Body": {
                   "transactions": [],
                   "txpackages": []
               },
               "FullHash": "0x41b12a0f3c23acd2e5b1a6ab435a3b2916abc4f5b6e1cf339e5eea84329da479",
               "Header": {
                   "parentHash": "0x020d08a7b92924fcff96a138f21265d24a99ae13addbc8aa42cb8f4bfe08cc63",
                   "round": 15663843021,
                   "sig": "FlIQD5tL9pNFkTnQJ2gEC6jLI8b1qtV41Oh0/jmsOn47Jf5otkOW2OzO4Qk5YILgza4SlCxiMUZiRpXX4kZAtA==",
                   "miner": "0xa04358d378cf97a933eb09b6014f4f118378e9f4",
                   "difficulty": 125601928805054582,
                   "height": 943,
                   "amount": 4840,
                   "gasLimit": 3670736464583360781,
                   "gasUsed": 0,
                   "stateHash": "0x63b6233c17bdb936354e1d4869d9c2034dc82ce8ca2d09aa11aba42ee452a8cc",
                   "txHash": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
                   "receiptsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
                   "parentFullHash": "0xe308587bbe23eec2ae9ddde7f52a727036c60e4fc8249493f95f1248c555cf8f",
                   "confirms": [
                       "0xce39c8a5fd8d224898aa19951505b3c657f9259650e0cfa45c5d13db282895d7",
                       "0x0aa8ddd7a5268b3b4636a4c0c18d4fbb99eb231c2362a3f8e561795501e3aaa8",
                       "0x7c0589ec94a1ad887a63cfbad225097f2d86db7f6628368c9c4b27714a7b86c5",
                       "0x09a55f4c0cfaa1dd3bb3fb70c920d150157f3e8200855cbeceb3d6cd3e663101",
                       "0x4c66dde446f65b46d703cd96337bfe42203e8e46925c04980f2ce3ced0455d11"
                   ],
                   "fullSig": "JtDbUl823+2kjblMdBhj0hBPl09PbviM6V9zrhAq4tlRVAZM6ceQ9BPAg2c0Y5SZ0q5wUEEIrYGkJYUpQRAARg==",
                   "minedTime": 1566384302104,
                   "hopCount": 0
               },
               "SimpleHash": "0x01286323b0ab5d744188ffa61c4ea8a8f6e205fd63e35c3843b244d44b2c13db"
           },
           ...,
       ]
   }

getBalance
''''''''''''''''''

GetBalance returns the amount of wei for the given address in the state of the given block.

Parameters:
"""""""""""
1. The hash of the address;
2. The hash of a specified block


Returns:
""""""""
1. The balance of the given address in the given block;
2. error, null if success;


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
               "method": "ftl_getBalance",
               "params": ["0xa04358d378cf97a933eb09b6014f4f118378e9f4", "0xd1c0f4f8e1ef3fb27bc19a3d0641f2e28cc61689340b10f73b3fdc65d0955fdf"]
   }

Responses:

Status: getBalance | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": "0x398df967c7600"
   }

.. UNTESTED

getStorageAt
'''''''''''''''''

Returns the value from a storage position at a given address.


Parameters:
"""""""""""
1. The address of the storage;
2. The table name of the expected position;
3. The key name of the expected position;
4. The hash of the block;


Returns:
""""""""
1. The value at this storage position;


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
               "method": "ftl_getStorageAt",
               "params": ["0xa04358d378cf97a933eb09b6014f4f118378e9f4", "0xd1c0f4f8e1ef3fb27bc19a3d0641f2e28cc61689340b10f73b3fdc65d0955fdf"]
   }

getCode
'''''''

GetCode returns the code of an contract address.


Parameters:
"""""""""""
1. The hash of the address;
2. The hash of a specified block;

Returns:
""""""""
1. The code of the address, should be "0x" if the address is not a contract address;


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
               "method": "ftl_getCode",
               "params": ["0xa04358d378cf97a933eb09b6014f4f118378e9f4", "0xd1c0f4f8e1ef3fb27bc19a3d0641f2e28cc61689340b10f73b3fdc65d0955fdf"]
   }

Responses:

Status: getCode | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": "0x"
   }

getContractOwner
''''''''''''''''''

GetContractOwner returns the owner of the given contract address.


Parameters:
"""""""""""
1. The hash of the contract address;


Returns:
""""""""
1. The address of the owner, should be all zero if the the given address is not a contract address;


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
               "method": "ftl_getContractOwner",
               "params": ["0xa04358d378cf97a933eb09b6014f4f118378e9f4"]
   }

Responses:

Status: getContractOwner | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": "0x0000000000000000000000000000000000000000"
   }

.. UNTESTED

getLogs
'''''''

Returns an array of all logs matching a given filter object.


Parameters:
"""""""""""
1. hash of the block;
2. beginning height of the blocks which will be searched from;
3. ending height of the blocks which will be searched from;
4. address of the contract;
5. An array of event topics;


Returns:
""""""""
1. An array of logs;


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
               "method": "ftl_getLogs",
               "params": [""]
   }

coinbase
''''''''

Coinbase is the address that mining rewards will be send to

Parameters:
"""""""""""
none


Returns:
""""""""
1. The coinbase of the node;


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
               "method": "ftl_coinbase",
               "params": ["0xa04358d378cf97a933eb09b6014f4f118378e9f4"]
   }

Responses:

Status: coinbase | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": "0xa04358d378cf97a933eb09b6014f4f118378e9f4"
   }

protocolVersion
''''''''''''''''''

ProtocolVersion returns the version of the Fractal protocol 

Parameters:
"""""""""""
none


Returns:
""""""""
1. The version of the protocol;


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
               "method": "ftl_protocolVersion",
               "params": []
   }

Responses:

Status: protocolVersion | Code: 200

.. code-block:: js

   {
       "jsonrpc": "2.0",
       "id": "1",
       "result": "0x2"
   }

chainId
'''''''

ChainID returns the Id of the chain.


Parameters:
"""""""""""
none


Returns:
""""""""
1. The Id of the chain;


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
               "method": "ftl_chainId",
               "params": []
   }

Responses:

Status: protocolVersion | Code: 200

.. code-block:: js

    {
        "jsonrpc": "2.0",
        "id": "1",
        "result": "0x3e7"
    }
