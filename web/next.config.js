module.exports = {
  images: {
    unoptimized: true,
  },
  async rewrites() {
    if (process.env.NODE_ENV === 'development')
      return [
        {
          source: '/api/:path*',
          destination: 'http://localhost:9235/api/:path*',
        },
      ];
    else return [];
  },
};
