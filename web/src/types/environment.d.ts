declare global {
  namespace NodeJS {
    interface ProcessEnv {
      NEXT_PUBLIC_ETH_TESTNET_EXPLORER: string;
      NEXT_PUBLIC_AVAX_TESTNET_EXPLORER: string;
      NEXT_PUBLIC_CIRCLE_ATTESTATION_API: string;
    }
  }
}

export {};
