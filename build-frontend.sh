#!/bin/bash

# Function to install or update Yarn
install_or_update_yarn() {
  if [ "$EUID" -eq 0 ]; then
    echo "Installing Yarn as superuser..."
    npm install -g yarn
  else
    echo "Installing Yarn as current user..."
    sudo npm install -g yarn
  fi
}

# Check if Docker is installed
if command -v docker >/dev/null 2>&1; then
  docker build --target build_frontend -t circuitbreaker-frontend-builder .
  CID=$(docker create circuitbreaker-frontend-builder)
  docker cp $CID:/webui-build .
  docker rm -f $CID
else
  # Check if Yarn is installed and if the version is greater than or equal to 1.10.0
  if command -v yarn >/dev/null 2>&1; then
    YARN_VERSION=$(yarn --version)
    if [[ $(echo "$YARN_VERSION >= 1.10.0" | bc -l) -eq 1 ]]; then
      echo "Yarn is installed with version $YARN_VERSION"
    else
      echo "Yarn version is less than 1.10.0, updating Yarn..."
      install_or_update_yarn
    fi
  else
    echo "Yarn is not installed, installing Yarn..."
    install_or_update_yarn
  fi

  # Run the Yarn commands
  cd ~/circuitbreaker/web
  yarn install --frozen-lockfile --network-timeout 1000000 && yarn build-export
fi
