interface APIError {
  code: number;
  message: string;
  details: {
    '@type': string;
    reason: string;
    domain: string;
  }[];
}
