# FAQ & Troubleshooting

## Connection Issues

### SSH connection failed — "Connection refused"

- Verify the SSH service is running on the target server
- Check that the IP address and port are correct
- Check whether the firewall allows the port

### SSH key authentication failed

- Verify the key file path and permissions are correct
- Ensure the server's `~/.ssh/authorized_keys` contains the corresponding public key
- Try password authentication to verify server reachability

### RDP white screen or freeze

- Check whether Remote Desktop is enabled on the target host
- Try lowering the resolution or color depth
- Windows Home edition does not support acting as an RDP server

### Serial connection unresponsive

- Confirm the serial port name is correct (COMx on Windows, /dev/ttyUSBx on Linux)
- Check that parameters such as baud rate match the device
- Try enabling "Local Echo" to verify whether input is being sent

## Feature Issues

### AI Assistant not responding

- Verify the API address and key are configured correctly
- Check whether the network can reach the API endpoint
- Review the connection test result on the AI model configuration page

### Cloud Sync failed

- Verify the Git repository URL and access token are correct
- Ensure the repository is private
- Check whether the network can reach the Git service

## More Help

If your issue is not listed here, please reach out through the following channels:

- [GitHub Issues](https://github.com/ys-ll/uniterm/issues)
