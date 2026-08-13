# Windows and WSL2 development setup

These steps preserve the host NVIDIA driver. The validated target is Windows with NVIDIA driver `591.91`, Ubuntu 24.04 under WSL2, CUDA Toolkit 13.1, Docker Desktop, and Go 1.26.5.

1. Enable WSL2 and install Ubuntu 24.04 from an elevated PowerShell terminal. Restart if Windows requests it.

   ```powershell
   wsl --install -d Ubuntu-24.04
   ```

2. Install Docker Desktop for the current Windows user. Select its WSL2 engine and enable integration for Ubuntu 24.04.

3. In Ubuntu, add NVIDIA's WSL-Ubuntu CUDA repository and install the toolkit package only:

   ```bash
   sudo apt-get update
   sudo apt-get install -y cuda-toolkit-13-1
   ```

   Never install `cuda-drivers` or a Linux NVIDIA display-driver package in WSL. The Windows host driver is projected into WSL.

4. Install Go 1.26.5 using the official archive and install build tools:

   ```bash
   sudo apt-get install -y build-essential cmake ninja-build git make curl ca-certificates
   # Download and verify the correct go1.26.5 linux-amd64 archive from go.dev/dl.
   ```

5. Keep the clone on the WSL filesystem for build performance:

   ```bash
   mkdir -p "$HOME/src"
   git clone https://github.com/dericsanandres/cudaops-platform "$HOME/src/cudaops-platform"
   cd "$HOME/src/cudaops-platform"
   ```

6. Verify the environment:

   ```bash
   nvidia-smi
   nvcc --version
   go version
   docker compose version
   docker run --rm --gpus all nvidia/cuda:13.1.2-runtime-ubuntu24.04 nvidia-smi
   ```

Expected results are CUDA Toolkit 13.1, Go 1.26.5, a working Compose v2 client, and the RTX 5060 visible both inside WSL and inside the CUDA container.

