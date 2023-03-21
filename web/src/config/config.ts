export const getConfig = (key?: keyof Config) => {
  if (typeof window !== 'undefined') {
    return key ? window.config[key] : window.config;
  }
  return undefined;
};
