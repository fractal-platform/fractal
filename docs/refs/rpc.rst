JSON-RPC API Reference
================================
*Draft*

All methods are classified into different namespaces.

Use the namespace as the prefix to call an RPC method.

Example:

.. code-block::

    method: addPeer
    namespace: admin

then,

.. code-block::

    method parameter in request body: `admin_addPeer`

You can also download the `Postman Collection <https://github.com/fractal-platform/fractal/tree/v0.1.x/docs/refs/rpc/fractal_rpc.postman_collection.json>`_ of sample requests.

The default Headers

+----------------+--------------------+
| Key            | Value              |
+================+====================+
| Content-Type   | application/json   |
+----------------+--------------------+

.. toctree::
   :maxdepth: 2
   :caption: JSON-RPC API List:

   rpc/admin
   rpc/ftl
   rpc/net
   rpc/packer
   rpc/txpool

