const removeLoader = () => {
  if (typeof window !== 'undefined') {
    const loader = document.getElementById('globalLoader');
    if (loader) {
      loader.style.opacity = '0';

      setTimeout(() => {
        loader.style.display = 'none';
      }, 250);
    }
  }
};

export default removeLoader;
