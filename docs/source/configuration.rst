#############################
CloudProvider Configuration
#############################

.. list-table:: Configuration Properties
   :widths: 20 20 60
   :header-rows: 1

   * - Property Name
     - Env Name
     - Description
   * - anexiaToken
     - ANEXIA_TOKEN
     - The token which is used to authenticate against the Anexia REST API.
   * - customerID
     - ANEXIA_CUSTOMER_ID
     - The customer prefix which needs to be prepended to evey Node objects name in order to find the corresponding
       in the Anexia API. If it's not set the CCM tries to discover this value automatically by looking at an already provisioned VM.
   * - clusterName
     - ANEXIA_CLUSTER_NAME
     - The name of the cluster. This can also be set via `--cluster-name`. This name will be part of the name when creating
       load balancer components on Anexia engine side. Also this name will be used for load balancer discovery (if
       `autoDiscoverLoadBalancer` is set.
   * - autoDiscoverLoadBalancer
     - ANEXIA_AUTO_DISCOVER_LOAD_BALANCER
     - If set the load balancer which is configured by the cloud controller manager will be discovered automatically.
   * - loadBalancerIdentifier
     - ANEXIA_LOAD_BALANCER_IDENTIFIER
     - The ID of the load balancer which should be configured by the cloud controller manager. This value will be ignored
       if `autoDiscoverLoadBalancer` is set.
   * - secondaryLoadBalancerIdentifiers
     - ANEXIA_SECONDARY_LOAD_BALANCER_IDENTIFIERS
     - The IDs of load balancers that should receive the same configuration as the load balancer that is configured by the
       cloud controller manager.
   * - loadBalancerPrefixIdentifiers
     - ANEXIA_LOAD_BALANCER_PREFIX_IDENTIFIERS
     - A list of identifiers of prefixes from which external IPs for LoadBalancer Services can be allocated by the cloud controller manager.
   * - autoDiscoveryTagPrefix
     - ANEXIA_AUTO_DISCOVERY_TAG_PREFIX
     - This prefix will be used together with the cluster name to find load balancer objects that should be configured.
       (only when auto discovery is enabled)

