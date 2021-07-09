########
Features
########

The CCM controller consists of many smaller sub controllers. currently the following controllers are implemented

NodeController
##############

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
