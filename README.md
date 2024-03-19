# findmyBLEscanner
**Purpose**
Bluetooth device scanner in two routines
- scanner routine
  Detects and maintain a list of local Bluetooth devices, passes them to the writer routine.
- screen writer routine.
  Receives a pre-sorted list of devices from the scanner, and prints them to a table.

Below is an AI generated breakdown of how this code works.
---
## Screen Writer
**Purpose**

Displays the output of the Bluetooth scanner in a neatly formatted table on the terminal screen. It does this by:

1. **Receiving device data:**  Reads the stream of scanned device data from a channel.
2. **Formatting the data:**  Prepares data rows for a table, including resolving company names and marking "Find My" devices.
3. **Table Management:** Uses the `go-pretty/table` library to create, style, and render the table.
4. **Clearing the Screen:** Ensures a clean display for each update.

**Types**

* **`screenWriter`** struct: Represents the component responsible for writing to the screen.
   * `wg`: WaitGroup for coordination.
   * `ptab`: A `table.Writer` instance from the `go-pretty/table` library.
   * `header`: The table's header row.
   * `quit`: Channel to signal stopping the writer.
   * `readPath`: Channel from which it receives device data 

**Functions**

* **`newWriter(...)`** Constructor for creating a `screenWriter`. Initializes the table writer and other settings.

* **`startWriter(...)`** Bootstraps the screen writer process. Creates a `screenWriter` and starts its execution loop.

* **`execute()`** The main loop of the `screenWriter`:
    * Waits for signals on the `quit` channel to stop.
    * Waits for device data on the `readPath` channel.
    * Calls `Write` to update the table when data arrives.

* **`Write(devs []devContent)`** Handles the table updates:
    * Iterates over the received device data.
    * Calls helper functions like `resolveCompanyIdent` and `isFindMyDevice` to process device information. 
    * Appends new rows to the table.
    * Clears the screen with `clearScreen` (you'll need to implement this function).
    * Renders the updated table.
    * Resets the table rows for the next update.

**Key Points**

* **External Libraries:** This code depends on:
    * `github.com/jedib0t/go-pretty/v6/table` for creating the table.
    * A function `clearScreen` (not provided here) which would be platform-specific for clearing the terminal.
* **Helper Functions** 
    * `resolveCompanyIdent` and `isFindMyDevice` are presumably used to extract more human-readable information from the device's manufacturer data. You would need the implementation of these functions and potentially a company ID map (`cmap`) to fully understand their logic.

**Let me know if you'd like any of the missing parts (like `clearScreen`, `resolveCompanyIdent`, etc.) explained or if you have questions about specific sections!** 


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
