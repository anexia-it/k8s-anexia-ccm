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


Load Balancer Discovery
-----------------------

Currently the cloud-controller-manageris not able to create a `LoadBalancer` resource inside the Anexia LBaaS module,
since this involves manual steps. However the CCM will configure these LoadBalancer resources (create Frontends,
FrontendBinds, Backend, BackendServers). In order to tell the cloud contrroller manager which LoadBalancer Object is to be
configured you have the following options.

#. Set the ID of the LoadBalancer directly via the `loadBalancerIdentifier` config value
#. Use the autodiscovery. The CCM will take the configured cluster name and the configured `autoDiscoveryTagPrefix` to properly
   to find LoadBalancers resource with a specific tag like this.

For more information about the configuration values see :ref:`CloudProvider Configuration`
