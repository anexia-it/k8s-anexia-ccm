####################
Installation (Helm)
####################

Prerequisites
#############


In order for the CCM to access the Anexia REST API you need to create k8s secret containing an access token.

.. code-block:: bash

    kubectl create secret generic anx-ccm-token --from-literal token=<YOUR_API_ACCESS_TOKEN>

If this secret is not present the CCM won't start.


Helm Install
#############

The CCM is deployed together with a cloud provider `config.json`. This `config.json` holds Anexia specific configuration
i.E your customer id. So your values file for the helm release should at least fill the `providerConfig` field.

.. code-block:: yaml

    providerConfig:
        customerPrefix: "<YOUR_CUSTOMER_ID>"

After creating this values file you can install the helm chart as ususal.

.. code-block:: bash

    helm install anexia-ccm ./chart --values <PATH_TO_YOUR_VALUES_FILE>

