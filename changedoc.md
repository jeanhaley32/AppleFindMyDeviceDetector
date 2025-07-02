# Refactoring Plan: AppleFindMyDeviceDetector

This document outlines a proposed refactoring for the AppleFindMyDeviceDetector project. The goal is to evolve the codebase from a single-binary script into a more robust, maintainable, and extensible Go application.

## Motivation for Change

The current implementation was highly successful as a learning project. It effectively proved the core concepts of BLE scanning, applying custom data filters, and concurrent processing.

As the project matures, the flat structure where all files reside in the root directory becomes a limitation. To improve the codebase, we will adopt a package-oriented architecture, which is standard practice in the Go community. This will provide a clear **separation of concerns**, making the application easier to understand, modify, and test.

## Proposed New Structure

The refactoring will organize the code into the following directory structure:

```
/AppleFindMyDeviceDetector/
├── cmd/
│   └── findmydetector/
│       └── main.go         # Main application entry point
├── internal/
│   ├── config/
│   │   └── config.go       # Application configuration
│   ├── corp/
│   │   └── identifier.go   # Logic for parsing company identifiers
│   ├── device/
│   │   └── device.go       # The core 'device' data model
│   ├── scanner/
│   │   └── scanner.go      # BLE scanning logic
│   └── writer/
│       └── writer.go       # Screen writer and table rendering logic
├── pkg/
│   └── util/
│       └── helpers.go      # General, non-project-specific utilities
├── go.mod
├── go.sum
├── company_identifiers.yaml
└── README.md
```

## Key Changes and Benefits

### 1. **Package-Based Organization**
- **Change:** Code will be grouped into distinct packages (`device`, `scanner`, `writer`, `config`, `corp`) based on its function.
- **Benefit:** This makes the codebase more modular and easier to navigate. Each package will have a clear, single responsibility.

### 2. **Separation of Concerns**
- **Change:**
    - The **`device`** package will define the core data structure.
    - The **`scanner`** package will be solely responsible for finding devices.
    - The **`writer`** package will be solely responsible for displaying devices.
- **Benefit:** This decoupling means we can change one part of the application with minimal impact on others. For example, we could change the output from a terminal table to a JSON API in the `writer` package without touching any of the Bluetooth scanning logic.

### 3. **Centralized Configuration**
- **Change:** Hardcoded values (like scan rates and timers) will be moved from constants into a `config` struct.
- **Benefit:** This makes the application's behavior easier to manage and modify. It is the first step toward allowing configuration via command-line flags or a file.

### 4. **Explicit Dependencies (No Global State)**
- **Change:** Global variables like the company identifier map (`cmap`) will be eliminated. Instead, dependencies will be passed explicitly to the components that need them (e.g., the `cmap` will be passed to the `writer`'s constructor).
- **Benefit:** This makes the data flow transparent and predictable. It also dramatically improves the testability of each component in isolation.

### 5. **Clear Application Entrypoint**
- **Change:** The `main.go` file will be moved to `cmd/findmydetector/`.
- **Benefit:** This is a standard Go convention that clearly separates the "runnable" part of the application from the "library" parts. The `main` function's role will be simplified to initializing and orchestrating the other packages.

By implementing these changes, we will create a solid foundation for future development, making the project more scalable and easier for others (or our future selves) to contribute to.
