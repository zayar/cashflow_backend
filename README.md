# PitiBooks — Cloud Accounting & Bookkeeping Platform

A multi-tenant accounting and bookkeeping SaaS platform built for small-to-medium businesses. The system provides double-entry bookkeeping, invoicing, payments, inventory management, and comprehensive financial reporting.

---

## Table of Contents

- [System Overview](#system-overview)
- [Architecture](#architecture)
- [Tech Stack](#tech-stack)
- [Module Breakdown](#module-breakdown)
- [Critical Business Flows](#critical-business-flows)
- [Project Structure](#project-structure)
- [Environment Variables](#environment-variables)
- [Local Development Setup](#local-development-setup)
- [Deployment](#deployment)
- [API Reference](#api-reference)
- [Known Issues & Improvement Roadmap](#known-issues--improvement-roadmap)

---

## System Overview

PitiBooks is a **multi-tenant accounting platform** that enables businesses to:

- **Manage Chart of Accounts** with customizable account types
- **Create and track Sales** (Quotes → Orders → Invoices → Payments)
- **Manage Purchases** (Purchase Orders → Bills → Supplier Payments)
- **Handle Inventory** with stock tracking, transfers, and valuation (FIFO/Weighted Average)
- **Process Banking Transactions** (deposits, transfers, owner contributions/drawings)
- **Generate Financial Reports** (P&L, Balance Sheet, Trial Balance, Ledgers, Aging Reports)
- **Support Multi-Currency** with exchange rate tracking and gain/loss calculations
- **Control Access** with role-based permissions per module

### Key Characteristics

| Aspect | Description |
|--------|-------------|
| **Multi-Tenant** | Each business (tenant) has isolated data via `business_id` |
| **Double-Entry** | All transactions create balanced journal entries (debits = credits) |
| **Event-Driven** | Domain events trigger async accounting postings via Pub/Sub |
| **Real-Time** | GraphQL API with DataLoader batching for efficient queries |
| **Localized** | Supports multiple timezones and fiscal year configurations |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              FRONTEND (React)                                │
│                     React + Apollo Client + Ant Design                       │
│                          Firebase Hosting                                    │
└─────────────────────────────────────────────────────────────────────────────┘
                                     │
                                     │ GraphQL (POST /query)
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           BACKEND API (Go + Gin)                             │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │   Session   │  │     Auth     │  │  DataLoader  │  │   GraphQL        │  │
│  │ Middleware  │→ │  Directive   │→ │  Middleware  │→ │   Resolvers      │  │
│  └─────────────┘  └──────────────┘  └──────────────┘  └──────────────────┘  │
│                                                              │               │
│                                                              ▼               │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │                         DOMAIN MODELS                                   │ │
│  │  Business │ Customer │ Supplier │ Product │ Invoice │ Bill │ Payment   │ │
│  │  Journal  │ Account  │ Expense  │ Stock   │ Transfer │ Credit Note     │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
│                              │                                               │
│                              ▼                                               │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                    PublishToAccounting()                                 ││
│  │         Writes PubSubMessageRecord + Publishes to Pub/Sub               ││
│  └─────────────────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────────────────┘
         │                              │
         │ MySQL (GORM)                 │ Google Cloud Pub/Sub
         ▼                              ▼
┌─────────────────┐          ┌─────────────────────────────────────────────────┐
│   Cloud SQL     │          │              ACCOUNTING WORKER                   │
│    (MySQL)      │          │  ┌─────────────────────────────────────────────┐│
│                 │          │  │   ProcessMessage() → ProcessWorkflow()      ││
│  - businesses   │          │  │                                              ││
│  - accounts     │◄─────────┤  │   workflow/invoiceWorkflow.go               ││
│  - invoices     │          │  │   workflow/customerPaymentWorkflow.go       ││
│  - bills        │          │  │   workflow/billWorkflow.go                  ││
│  - payments     │          │  │   workflow/manualJournalWorkflow.go         ││
│  - journals     │          │  │   ... (20+ workflow handlers)               ││
│  - stocks       │          │  │                                              ││
│  - etc.         │          │  │   Creates: AccountJournal + AccountTransactions││
│                 │          │  │   Updates: Daily Balance Projections        ││
└─────────────────┘          │  └─────────────────────────────────────────────┘│
                             └─────────────────────────────────────────────────┘
         │
         │ Redis
         ▼
┌─────────────────┐          ┌─────────────────────────────────────────────────┐
│  Memorystore    │          │           FIREBASE FUNCTIONS                    │
│    (Redis)      │          │  ┌─────────────────────────────────────────────┐│
│                 │          │  │   onProductUpdate (Pub/Sub trigger)         ││
│  - Sessions     │          │  │   → Syncs products to external integrations ││
│  - APQ Cache    │          │  │      (e.g., Cashflow POS)                   ││
│  - Role Paths   │          │  └─────────────────────────────────────────────┘│
│  - Locks        │          └─────────────────────────────────────────────────┘
│  - Counters     │
└─────────────────┘
```

### Request Flow

1. **Authentication**: Client sends `token` header → `SessionMiddleware` validates via Redis → sets `username` in context
2. **Authorization**: `@auth` directive resolves user → checks role permissions → sets `businessId`, `userId`, `userName` in context
3. **Data Loading**: `LoaderMiddleware` initializes DataLoaders for N+1 query prevention
4. **Mutation Processing**: Domain model creates/updates records → calls `PublishToAccounting()`
5. **Async Posting**: Pub/Sub message triggers worker → workflow creates journal entries → updates balances

---

## Tech Stack

### Backend (Go)

| Component | Technology |
|-----------|------------|
| **Language** | Go 1.21 |
| **Framework** | Gin HTTP framework |
| **GraphQL** | gqlgen (code-first schema) |
| **ORM** | GORM (MySQL driver) |
| **Messaging** | Google Cloud Pub/Sub |
| **Caching** | Redis (go-redis/v9) |
| **Validation** | go-playground/validator |
| **Decimal Math** | shopspring/decimal |
| **Tracing** | OpenTelemetry |
| **Logging** | Logrus |

### Frontend (React)

| Component | Technology |
|-----------|------------|
| **Framework** | React 18 |
| **Build Tool** | Vite |
| **GraphQL Client** | Apollo Client |
| **UI Library** | Ant Design 5.x |
| **Charting** | Chart.js + react-chartjs-2 |
| **PDF Generation** | @react-pdf/renderer, jsPDF |
| **Excel Export** | ExcelJS |
| **Internationalization** | react-intl |
| **Routing** | react-router-dom v6 |

### Firebase Functions (TypeScript)

| Component | Technology |
|-----------|------------|
| **Runtime** | Node.js 20 |
| **Framework** | Firebase Functions v2 |
| **HTTP Client** | Axios |
| **Purpose** | External integrations (POS sync) |

### Infrastructure (GCP)

| Component | Service |
|-----------|---------|
| **Compute** | Cloud Run (recommended) |
| **Database** | Cloud SQL (MySQL 8.0) |
| **Cache** | Memorystore (Redis) |
| **Messaging** | Cloud Pub/Sub |
| **Secrets** | Secret Manager |
| **Logging** | Cloud Logging |
| **Monitoring** | Cloud Monitoring |
| **Frontend Hosting** | Firebase Hosting |

---

## Module Breakdown

### Core Business Modules

| Module | Description | Key Models |
|--------|-------------|------------|
| **Tenant Management** | Business registration, settings, fiscal year, timezone | `Business`, `Branch`, `Warehouse` |
| **User & Access Control** | Authentication, roles, module-level permissions | `User`, `Role`, `Module`, `RoleModule` |
| **Chart of Accounts** | Account types, system accounts, custom accounts | `Account`, `MoneyAccount` |
| **Customers** | Customer master, addresses, contacts, opening balances | `Customer`, `BillingAddress`, `ShippingAddress`, `ContactPerson` |
| **Suppliers** | Supplier master, addresses, contacts, opening balances | `Supplier` |
| **Products & Inventory** | Products, variants, groups, categories, units | `Product`, `ProductVariant`, `ProductGroup`, `ProductCategory`, `ProductUnit` |
| **Stock Management** | Stock movements, transfers, adjustments, valuation | `Stock`, `StockSummary`, `StockHistory`, `TransferOrder`, `InventoryAdjustment` |

### Transaction Modules

| Module | Description | Key Models |
|--------|-------------|------------|
| **Sales Orders** | Quotes and orders before invoicing | `SalesOrder`, `SalesOrderDetail` |
| **Sales Invoices** | Customer invoices with line items | `SalesInvoice`, `SalesInvoiceDetail` |
| **Customer Payments** | Payment receipts, multi-invoice allocation | `CustomerPayment`, `PaidInvoice` |
| **Credit Notes** | Customer refunds and credits | `CreditNote`, `CreditNoteDetail`, `CustomerCreditInvoice` |
| **Purchase Orders** | Supplier purchase orders | `PurchaseOrder`, `PurchaseOrderDetail` |
| **Bills** | Supplier invoices (bills) | `Bill`, `BillDetail` |
| **Supplier Payments** | Payments to suppliers | `SupplierPayment`, `SupplierPaidBill` |
| **Supplier Credits** | Supplier refunds and credits | `SupplierCredit`, `SupplierCreditBill` |
| **Expenses** | Direct expense recording | `Expense`, `ExpenseDetail` |
| **Banking** | Deposits, transfers, owner transactions | `BankingTransaction` |
| **Manual Journals** | Direct journal entries | `Journal`, `JournalTransaction` |

### Accounting Engine

| Module | Description | Key Models |
|--------|-------------|------------|
| **Journal Entries** | Auto-generated double-entry postings | `AccountJournal`, `AccountTransaction` |
| **Balance Tracking** | Daily/current balance projections | `AccountCurrencyDailyBalance` |
| **Opening Balances** | Migration/opening balance entries | `OpeningBalance` |
| **Transaction Locking** | Period close and lock dates | `TransactionLockingRecord` |
| **Currency Exchange** | Multi-currency rates and conversions | `Currency`, `CurrencyExchange` |

### Reporting

| Category | Reports |
|----------|---------|
| **Financial Statements** | Profit & Loss, Balance Sheet, Trial Balance, Cash Flow |
| **Ledgers** | General Ledger, Account Transactions |
| **Receivables** | Customer Balances, AR Aging, Invoice Details, Customer Statements |
| **Payables** | Supplier Balances, AP Aging, Bill Details, Supplier Statements |
| **Inventory** | Stock Summary, Inventory Valuation, Stock Movement, FIFO Cost Layers |
| **Banking** | Account Statements, Bank Reconciliation |
| **Tax** | Tax Summary, Realized/Unrealized FX Gain/Loss |

---

## Critical Business Flows

### 1. Invoice Posting Flow

```
CreateSalesInvoice (mutation)
    │
    ├── Validate: customer, branch, warehouse, products, stock availability
    ├── Calculate: line totals, discounts, taxes, grand total
    ├── Insert: sales_invoices + sales_invoice_details
    ├── Update: stock_summary (reduce available qty)
    ├── PublishToAccounting(tx, ..., AccountReferenceTypeInvoice, ...)
    │       ├── Insert: pub_sub_message_records (outbox)
    │       └── Publish: Pub/Sub message
    └── Commit transaction
           │
           ▼ (async)
    ProcessInvoiceWorkflow (worker)
        ├── CreateInvoice() → AccountJournal + AccountTransactions
        │     DR  Accounts Receivable
        │     CR  Sales Revenue
        │     CR  Tax Payable (if applicable)
        │     DR  Cost of Goods Sold (for inventory items)
        │     CR  Inventory
        ├── ProcessOutgoingStocks() → StockHistory + FIFO layers
        ├── UpdateBalances() → AccountCurrencyDailyBalance
        └── Mark message as processed
```

### 2. Customer Payment Flow

```
CreateCustomerPayment (mutation)
    │
    ├── Validate: customer, invoices, deposit account
    ├── Allocate: payment amounts to invoices
    ├── Insert: customer_payments + paid_invoices
    ├── Update: sales_invoices (paid amounts, remaining balance, status)
    ├── PublishToAccounting(tx, ..., AccountReferenceTypeCustomerPayment, ...)
    └── Commit transaction
           │
           ▼ (async)
    ProcessCustomerPaymentWorkflow (worker)
        ├── CreateCustomerPayment() → AccountJournal + AccountTransactions
        │     DR  Bank/Cash Account
        │     CR  Accounts Receivable
        │     DR/CR  Exchange Gain/Loss (if multi-currency)
        │     DR  Bank Charges (if applicable)
        ├── Insert: banking_transactions (for cash/bank accounts)
        ├── UpdateBalances()
        ├── UpdateBankBalances()
        └── Mark message as processed
```

### 3. Manual Journal Flow

```
CreateJournal (mutation)
    │
    ├── Validate: accounts, branch, currency, lock date
    ├── Validate: sum(debits) == sum(credits)
    ├── Insert: journals + journal_transactions
    ├── PublishToAccounting(tx, ..., AccountReferenceTypeJournal, ...)
    └── Commit transaction
           │
           ▼ (async)
    ProcessManualJournalWorkflow (worker)
        ├── CreateManualJournal() → AccountJournal + AccountTransactions
        ├── Insert: banking_transactions (for cash/bank lines)
        ├── UpdateBalances()
        ├── UpdateBankBalances()
        └── Mark message as processed
```

---

## Project Structure

```
backend/
├── server.go                 # Main entry point (Gin + GraphQL setup)
├── accountingWorkflow.go     # Pub/Sub message processor
├── config/
│   ├── database.go           # MySQL/GORM connection
│   ├── redisDb.go            # Redis connection + helpers
│   ├── gPubSub.go            # Pub/Sub client + publisher
│   └── logrus.go             # Logging configuration
├── directives/
│   └── auth.go               # @auth GraphQL directive
├── graph/
│   ├── schema.graphqls       # GraphQL schema definition
│   ├── schema.resolvers.go   # Generated + custom resolvers
│   ├── resolver.go           # Resolver struct
│   ├── generated.go          # gqlgen generated code
│   └── helper.go             # Resolver helpers
├── middlewares/
│   ├── SessionMiddleware.go  # Token → username resolution
│   ├── LoaderMiddleware.go   # DataLoader initialization (80+ loaders)
│   └── ...                   # Other middleware files
├── models/
│   ├── base.go               # PublishToAccounting, common helpers
│   ├── business.go           # Tenant model + CRUD
│   ├── salesInvoice.go       # Invoice model + CRUD
│   ├── customerPayment.go    # Payment model + CRUD
│   ├── accountTransaction.go # Journal entries model
│   ├── stock.go              # Stock/inventory models
│   ├── enums.go              # Enum definitions
│   └── ...                   # ~80 model files
├── workflow/
│   ├── mainWorkflow.go       # Common workflow helpers
│   ├── invoiceWorkflow.go    # Invoice posting logic
│   ├── customerPaymentWorkflow.go
│   ├── billWorkflow.go
│   ├── manualJournalWorkflow.go
│   └── ...                   # 25 workflow handlers
├── utils/
│   ├── helper.go             # General utilities
│   ├── contextHelper.go      # Context value helpers
│   ├── validateHelper.go     # Validation utilities
│   └── ...
├── go.mod
├── go.sum
├── Dockerfile
└── bitbucket-pipelines.yml   # CI/CD configuration

functions/
├── functions/
│   ├── src/
│   │   ├── index.ts          # Firebase Functions entry
│   │   ├── pubsub/
│   │   │   ├── index.ts
│   │   │   └── onProductUpdate.ts   # Product sync trigger
│   │   ├── integrations/
│   │   │   └── piti.ts       # External POS integration
│   │   └── helpers/
│   │       └── httpClient.ts # HTTP client wrapper
│   ├── package.json
│   └── tsconfig.json
└── firebase.json

frontend/
├── src/
│   ├── App.js                # Main app component
│   ├── pages/                # Page components
│   │   ├── sales/            # Invoices, orders, payments
│   │   ├── purchases/        # Bills, POs, supplier payments
│   │   ├── products/         # Inventory management
│   │   ├── banking/          # Bank transactions
│   │   ├── accountant/       # Journals, chart of accounts
│   │   ├── reports/          # All reports
│   │   └── settings/         # Business settings
│   ├── components/           # Reusable components
│   ├── graphql/
│   │   ├── queries/          # GraphQL query definitions
│   │   └── mutations/        # GraphQL mutation definitions
│   ├── config/
│   │   ├── Theme.js          # Ant Design theme
│   │   └── Constants.js      # App constants
│   ├── locales/              # i18n translations
│   │   ├── en.json
│   │   └── mm.json           # Myanmar language
│   └── utils/
├── package.json
├── vite.config.js
└── firebase.json
```

---

## Environment Variables

### Backend

```bash
# Database
DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=your_password
DB_NAME_2=pitibooks

# Redis
REDIS_ADDRESS=localhost:6379

# API
API_PORT_2=8080

# Pub/Sub
PUBSUB_TOPIC=PitiAccounting
PUBSUB_SUBSCRIPTION=PitiAccountingSubscription

# Authentication
TOKEN_HOUR_LIFESPAN=24

# Optional
GO_ENV=development
GORM_LOG=gorm.log
```

### Frontend

```bash
VITE_GRAPHQL_ENDPOINT=http://localhost:8080/query
VITE_FIREBASE_API_KEY=your_key
VITE_FIREBASE_AUTH_DOMAIN=your_domain
VITE_FIREBASE_PROJECT_ID=your_project
```

---

## Local Development Setup

### Prerequisites

- Go 1.21+
- Node.js 20+
- MySQL 8.0
- Redis 6+
- Google Cloud SDK (for Pub/Sub emulator, optional)

### Backend Setup

```bash
cd backend

# Install dependencies
go mod download

# Set up environment
cp .env.example .env
# Edit .env with your database credentials

# Create database
mysql -u root -p -e "CREATE DATABASE pitibooks;"

# Run migrations (handled automatically by GORM on startup)
go run server.go

# Server starts at http://localhost:8080
# GraphQL Playground: http://localhost:8080/
```

### Frontend Setup

```bash
cd frontend

# Install dependencies
npm install
# or
yarn install

# Set up environment
cp .env.example .env.local
# Edit with your API endpoint

# Run development server
npm run dev
# or
yarn dev

# Opens at http://localhost:5173
```

### Firebase Functions (Optional)

```bash
cd functions/functions

# Install dependencies
npm install

# Build
npm run build

# Run emulator
npm run serve
```

---

## Deployment

### Backend (Cloud Run)

```bash
# Build container
gcloud builds submit --tag gcr.io/PROJECT_ID/books-backend

# Deploy to Cloud Run
gcloud run deploy books-backend \
  --image gcr.io/PROJECT_ID/books-backend \
  --platform managed \
  --region asia-southeast1 \
  --allow-unauthenticated \
  --set-env-vars "DB_HOST=/cloudsql/PROJECT:REGION:INSTANCE" \
  --add-cloudsql-instances PROJECT:REGION:INSTANCE \
  --set-secrets "DB_PASSWORD=db-password:latest"
```

### Frontend (Firebase Hosting)

```bash
cd frontend

# Build production bundle
npm run build

# Deploy to Firebase
firebase deploy --only hosting
```

---

## API Reference

### GraphQL Endpoint

```
POST /query
Content-Type: application/json
token: <user_session_token>
```

### Key Queries

| Query | Description |
|-------|-------------|
| `getBusiness` | Current business details |
| `getSalesInvoices` | List invoices with filters |
| `getSalesInvoice(id)` | Single invoice with details |
| `getBills` | List supplier bills |
| `getAccounts` | Chart of accounts |
| `getTrialBalance(...)` | Trial balance report |
| `getProfitAndLoss(...)` | P&L statement |
| `getBalanceSheet(...)` | Balance sheet |
| `getCustomerBalances(...)` | AR aging report |

### Key Mutations

| Mutation | Description |
|----------|-------------|
| `login(username, password)` | User authentication |
| `createSalesInvoice(input)` | Create new invoice |
| `updateSalesInvoice(id, input)` | Update invoice |
| `deleteSalesInvoice(id)` | Delete/void invoice |
| `createCustomerPayment(input)` | Record payment |
| `createJournal(input)` | Manual journal entry |
| `createProduct(input)` | Add new product |

---

## Known Issues & Improvement Roadmap

### Critical Issues (Phase 0 — Fix Immediately)

| Issue | Risk | Fix |
|-------|------|-----|
| **Hardcoded GCP credentials** | Security breach, compliance violation | Move to Secret Manager + Workload Identity |
| **Publish-before-commit** | Lost events, inconsistent balances | Implement transactional outbox pattern |
| **Ack on error** | Permanent message loss | Return non-2xx for transient failures |
| **Redis TTL idempotency** | Duplicate postings after 30 min | Use DB-backed idempotency table |
| **Missing tenant filters** | Cross-tenant data leaks | Audit and fix all queries |

### Architecture Improvements (Phase 1+)

| Improvement | Benefit |
|-------------|---------|
| **Immutable ledger** | Audit trail, reversals instead of deletes |
| **Pub/Sub ordering keys** | Per-tenant sequential processing |
| **Period close controls** | Centralized posting gate |
| **Structured error taxonomy** | Better debugging and user feedback |
| **Correlation IDs** | End-to-end request tracing |
| **Dead-letter queue + replay** | Handling failed messages |

### Future Enhancements

- [ ] BigQuery for analytics/reporting
- [ ] Bank feed integration (Open Banking)
- [ ] Mobile app (React Native)
- [ ] Multi-company consolidation
- [ ] Budgeting and forecasting
- [ ] Approval workflows

---

## License

Proprietary — MMDataFocus

## Support

For technical support, contact the development team.
