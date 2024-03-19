# findmyBLEscanner
**Purpose**
Bluetooth device scanner broken into two parts. 

## BLE Scanner
This code implements a Bluetooth Low Energy (BLE) device scanner. Its essential functions are:

1. **Scanning for BLE Devices:** Discovers nearby BLE devices and gathers their data.
2. **Storing Device Information:** Maintains a list of discovered devices along with information like their addresses, manufacturer data, and the time they were last seen.
3. **Transmitting Device Data:** Regularly sends the collected device data downstream for further processing.
4. **Managing Old Devices:**  Removes devices from the list if they haven't been seen within a specified time threshold.

**Package and Imports**

* **`main`:**  The main package for the executable program.
* **`fmt`:**  Standard input/output (I/O) formatting.
* **`log`:**  Simple logging.
* **`reflect`:**  Runtime type inspection.
* **`sort`:**  Sorting.
* **`sync`:**  Synchronization primitives (e.g., WaitGroup).
* **`time`:**  Time-related functions.
* **`tinygo.org/x/bluetooth`:**  Bluetooth library (assumed to be TinyGo-specific).

**Constants**

Defines time intervals and buffer sizes for the scanning and data processing:

* **`scanRate`:** How often to start a new scan.
* **`scanBufferSize`:** Capacity of the channel receiving scan results.
* **`scanLength`:**  How long each scan lasts.
* **`writeTime`:** How often to send the collected devices to the ingest path.
* **`trimTime`:** How often to remove old devices from the storage.
* **`oldestDevice`:** Maximum age for devices in storage.

**Types**

* **`scanner` struct:** Represents the BLE scanner object.
  * `wg`: WaitGroup for coordination.
  * `adptr`: Bluetooth adapter.
  * `devices`: Map storing device data (Key: UUID, Value: map[UUID]devContent)
  * `count`: Count of discovered devices.
  * `start`:  Timestamp of when the scan began.
  * `quit`: Channel to signal stopping the scan.
  * `ingPath`: Channel to send device data.
* **`DevContentList`:** A sortable slice of `devContent` structs.
* **`ingestPath`:**  A channel for transmitting the discovered device data.
* **`devContent`** struct:** Represents information about a single BLE device
  * `id`: UUID of the device.
  * `manufacturerData`: Raw manufacturer data.
  * `localName`: Device's advertised name.
  * `companyIdent`:  Company identifier extracted from manufacturer data.
  * `lastSeen`: Timestamp when the device was last observed.

**Functions**

* **`scan(returnPath chan bluetooth.ScanResult)`:**  The core scanning function.  Performs repeated scans and sends results on the `returnPath` channel.
* **`newScanner(...)`:** A constructor to create a new `scanner` object.
* **`startScan()`:** Starts the scanner's main loop (scanning, writing data, and trimming).
* **`startBleScanner(wg *sync.WaitGroup, ingPath ingestPath, q chan any)`:** Bootstraps the scanner, sets up the Bluetooth adapter, and starts the scanning process.
* **`scanlog(s string)`:** Simple logging function.
* **`TrimMap()`:** Removes stale device entries from the `devices` map.
* **`sortAndPass()`:**  Sorts the devices by ID and sends them on the `ingPath`.

Let me know if you'd like any part explained in more detail! 
