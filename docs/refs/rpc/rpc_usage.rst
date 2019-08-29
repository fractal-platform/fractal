JSON RPC
========

All methods are classified into 5 different namespaces:

.. toctree::
   :maxdepth: 1


   admin
   ftl
   net
   packer
   txpool

Use the namespace as the prefix to call an RPC method.

Example:

.. code-block::

    method: addPeer
    namespace: admin

then,

.. code-block::

    method parameter in request body: `admin_addPeer`

You can also download the `Postman Collection <http://www.baidu.com>`_ of sample requests.

The default Headers
'''''''''''''''''''

+----------------+--------------------+
| Key            | Value              |
+================+====================+
| Content-Type   | application/json   |
+----------------+--------------------+



