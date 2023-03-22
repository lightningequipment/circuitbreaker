module.exports = {
  images: {
    unoptimized: true,
  },
  async rewrites() {
    if (process.env.NODE_ENV === 'development')
      return [
        {
          source: '/api/:path*',
          destination: 'http://127.0.0.1:9235/api/:path*',
        },
      ];
    else return [];
  },
};
