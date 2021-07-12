####################
Installation (Helm)
####################

Prerequisites
#############


In order for the CCM to access the Anexia REST API you can obtain an access token from your `profile <https://engine.anexia-it.com/profile>`__.


Helm Install
#############

The CCM is deployed together with a cloud provider `config.yaml`. This `config.yaml` holds Anexia specific configuration
i.E your customer id and the access token.
So your values file for the helm release should at least fill the `providerConfig` field.

.. code-block:: yaml

    providerConfig:
        customerPrefix: "<YOUR_CUSTOMER_ID>"
        anexiaToken: "<YOUR_ANEXIA_ACCESS_TOKEN>"

After creating this values file you can install the helm chart as ususal.

.. code-block:: bash

    helm install anexia-ccm ./chart --values <PATH_TO_YOUR_VALUES_FILE>

