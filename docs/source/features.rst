########
Features
########

This chapter will give you an overview of the components that are currently included in the CCM controller. And give you some
hints on what to consider when you run the CCM in your cluster.

Node Controller
################

The node controller is responsible for creating Node objects when new servers are created in your cloud infrastructure.
The node controller obtains information about the hosts running inside your tenancy with the cloud provider.
The node controller performs the following functions:

#. Initialize a Node object for each server that the controller discovers through the cloud provider API.
#. Annotating and labelling the Node object with cloud-specific information, such as the region the node is deployed into and the resources (CPU, memory, etc) that it has available.
#. Obtain the node's hostname and network addresses.
#. Verifying the node's health. In case a node becomes unresponsive, this controller checks with your cloud provider's API to see if the server has been deactivated / deleted / terminated. If the node has been deleted from the cloud, the controller deletes the Node object from your Kubernetes cluster.


Limitations
-----------

Currently the CCM takes the first VLAN it finds on the VM to initialise the `Node` objects `InternalIP`. So having multiple
VLANs connected to VM can lead to weird behaviour.


Service Controller
##################

The service controller is responsible to ensure that services of type `LoadBalancer` are working together with the Anexia LBaaS
module.
The service controller performs the following functions:

#. Reacting to changes to the `Service` resource (when they are of type `LoadBalancer`).
#. Reacting to changes to the `Node` resource.
#. Creating a configuration from the `Service` object and the current `Node` Objects.
#. Configuring the specified load balancer. (See load balancer discovery)


Annotations
-----------

Like most cloud providers, we allow configuring some features via annotations on the objects:

#. ``lbaas.anx.io/external-ip-families: IPv4,IPv6``

   Allows configuring the families for external IP address allocated for a LoadBalancer service. This list does not
   have to match the families of the service; configuring "external IPv6, internal IPv4" is perfectly fine.

   A given IP family is only allowed once, so "``IPv4``", "``IPv6``", "``IPv4,IPv6``" and "``IPv6,IPv4``" are the only
   possible values. The order given is not relevant, the value is case-sensitive and no spaces are allowed though.

   If this annotation is not set, ``.spec.ipFamilies`` of the service is used instead, meaning a service internally
   being dual-stack is dual-stack externally, too.

#. ``lbaas.anx.io/load-balancer-proxy-pass-hostname: <RFC 1123-valid hostname>``

   Allows to set the hostname for a given service instead of its IP addresses.
   This is necessary in order to support the PROXY protocol, which otherwise
   would be broken due to hairpinning of kube-proxy.

   It can be set to any hostname that is valid according to `RFC 1123 <https://www.rfc-editor.org/rfc/rfc1123>`_.

   The actual value for the hostname does not have an effect on the actual routing.
   Technically, it just sets the `hostname` on the status of the service.

   If you want to expose multiple ingresses, like `first.example.com` and `second.example.com`, you can do so without
   any problems, independent of the value of the annotation.

PROXY protocol support
----------------------

In order to enable PROXY protocol support, there are two things to be done:

# Create a support ticket, so that we can enable it on our side. This has to be done manually for now.
# Set the `lbaas.anx.io/load-balancer-proxy-pass-hostname` on the `Service` of type `LoadBalancer` to *any* hostname (see documentation above).

Example: stefanprodan/podinfo together with nginx Ingress Controller
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

The `stefanprodan/podinfo` provides an example workflow to test the ingress stack as intended.

.. code-block::

   # Install podinfo
   kubectl apply -k github.com/stefanprodan/podinfo//kustomize

   # Install nginx ingress controller
   kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.12.0/deploy/static/provider/cloud/deploy.yaml

   # Enable proxy protocol
   kubectl patch -p '{"data":{"use-proxy-protocol":"true"}}' configmaps ingress-nginx-controller

   # Annotate the ingress service with the hostname (replace test.anx.io with the actual hostname)
   kubectl annotate service -n ingress-nginx ingress-nginx-controller lbaas.anx.io/load-balancer-proxy-pass-hostname=test.anx.io

   # Expose the podinfo via a new Ingress resource
   kubectl create ingress podinfo --class=nginx --rule="test.anx.io/*=podinfo:http"

Load Balancer Discovery
-----------------------

Currently the cloud-controller-manager is not able to create a `LoadBalancer` resource inside the Anexia LBaaS module,
since this involves manual steps. However the CCM will configure these LoadBalancer resources (create Frontends,
FrontendBinds, Backend, BackendServers). In order to tell the cloud contrroller manager which LoadBalancer Object is to be
configured you have the following options.

#. Set the ID of the LoadBalancer directly via the `loadBalancerIdentifier` config value
#. Use the autodiscovery. The CCM will take the configured cluster name and the configured `autoDiscoveryTagPrefix` to properly
   to find LoadBalancers resource with a specific tag like this.

For more information about the configuration values see :ref:`CloudProvider Configuration`
