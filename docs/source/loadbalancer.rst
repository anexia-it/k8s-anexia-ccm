######################
LoadBalancer internals
######################

LBaaS resources
---------------

For a given Kubernetes Service of type LoadBalancer, we need to manage numerous Anexia LBaaS resources

#. one Frontend and Backend per port in the Service
#. one FrontendBind per Frontend + external IP address
#. one BackendServer per Backend + Kubernetes node

These resources have a name suffix of ``.$serviceName.$serviceNamespace.$clusterName``, with resource-specific
data before:

#. Frontend and Backend use the name of the port (``http.test-service.default.some-cluster``)
#. FrontendBinds use the address family (``v4``/``v6``) and name of the port  (``v4.http.test-service.default.some-cluster``)
#. BackendServers use the name of the node and name of the port (``machine-deploy-a-2345413453-0843q.test-service.default.some-cluster``)

LBaaS resources are tagged  with ``anxccm-svc-uid=$service-uid`` (``$service-uid`` is ``.metadata.uid``) to find
them later.


Reconcilation
-------------

#. retrieve resources tagged with the service UID
    #. filter resources by the LoadBalancer they belong to as a given Service can be provisioned onto many LBaaS LoadBalancers and still have the same tag
    #. Frontends and Backends are directly attached to their LoadBalancer
    #. FrontendBinds and BackendServers are checked after all resources are retrieved and kept in the working set if their Frontend/Backend is in the working set
#. in a loop over the resource types (in the order Backend, Frontend, FrontendBind, BackendServer), until something needs to be created:
    #. determine the target set of resources
    #. compare with existing resources, creating a list of resources to create and a list of resources to destroy
#. destroy any resources that are not needed anymore
#. create new resources
#. if something was destroyed or created: go to step 1


