#############################
CloudProvider Configuration
#############################

.. list-table:: Configuration Properties
   :widths: 50 50 100
   :header-rows: 1

   * - Property Name
     - Env Name
     - Description
   * - anexiaToken
     - ANEXIA_TOKEN
     - The token which is used to authenticate against the Anexia REST API
   * - customerID
     - ANEXIA_CUSTOMER_ID
     - The customer prefix which needs to be prepended to evey Node objects name in order to find the corresponding
       in the Anexia API. If it's not set the CCM tries to discover this value automatically by looking at an already provisioned VM.

