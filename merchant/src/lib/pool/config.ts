// Runtime configuration for the merchant app. The app reaches the layer's
// product_fit read with the merchant key and nothing else: these are the
// only environment names it reads. Nothing here configures any other tier,
// and the companion proof test pins that.

/** The environment name holding the merchant key issued for product_fit. */
export const merchantKeyEnvVar = 'MEMENTO_MERCHANT_KEY';

/** The environment name holding the layer's public base URL. */
export const layerUrlEnvVar = 'MEMENTO_LAYER_URL';

/** Every environment name the merchant app reads. Nothing else is read. */
export const merchantEnvVars = [merchantKeyEnvVar, layerUrlEnvVar] as const;
