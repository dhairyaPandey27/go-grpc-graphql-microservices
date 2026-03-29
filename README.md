# go-grpc-graphql-microservices

A production-style microservices backend for an e-commerce platform, written entirely in Go. The system is decomposed into three independently deployable gRPC services -- Account, Catalog, and Order -- each owning its own database and exposing a well-defined Protocol Buffers (proto3) API. A fourth component, the GraphQL API gateway, sits in front of all three services and provides a single, unified HTTP endpoint that clients can query using GraphQL. The gateway translates every incoming GraphQL query and mutation into one or more gRPC calls to the appropriate backend services, aggregates the results, and returns a single JSON response. Inter-service communication (for example, the Order service validating an account or fetching product details) also happens over gRPC. The Account and Order services store their data in PostgreSQL, while the Catalog service uses Elasticsearch to support full-text product search. Every service is containerized using multi-stage Docker builds (Alpine-based, with vendored dependencies) and the entire stack -- all four application containers plus the three database containers -- is orchestrated via a single Docker Compose file, making it possible to bring up the full system with one command.

---

## Architecture Overview

```
                     +-------------------+
                     |   Client / App    |
                     +--------+----------+
                              |
                         HTTP :8000
                              |
                     +--------v----------+
                     |  GraphQL Gateway   |
                     |  (gqlgen / HTTP)   |
                     +--+------+------+---+
                        |      |      |
               gRPC     | gRPC |      | gRPC
              :8080     |:8080 |      | :8080
                        |      |      |
              +---------+  +---+---+  +----------+
              | Account |  |Catalog|  |  Order   |
              | Service |  |Service|  | Service  |
              +----+----+  +---+---+  +--+---+---+
                   |           |         |   |
              Postgres   Elasticsearch   | Postgres
            (account_db)  (catalog_db)   |  (order_db)
                                         |
                              gRPC calls to Account
                              and Catalog services
```

- Clients interact exclusively through the **GraphQL gateway** exposed on port 8000.
- The GraphQL gateway translates incoming GraphQL queries and mutations into **gRPC calls** to the backend microservices.
- The **Order service** depends on both the Account and Catalog services (it validates accounts and fetches product details when creating orders).
- Each service owns its own database.

---

## Project Structure

```
go-grpc-graphql-microservices/
|
|-- account/                    # Account microservice
|   |-- cmd/account/main.go     # Entrypoint
|   |-- pb/                     # Generated protobuf + gRPC Go files
|   |   |-- account.pb.go
|   |   |-- account_grpc.pb.go
|   |-- account.proto           # Protobuf service definition
|   |-- client.go               # gRPC client wrapper
|   |-- server.go               # gRPC server implementation
|   |-- service.go              # Business logic layer (Service interface)
|   |-- repository.go           # Data access layer (PostgreSQL)
|   |-- up.sql                  # Database migration / init schema
|   |-- app.dockerfile          # Multi-stage build for the service binary
|   |-- db.dockerfile           # PostgreSQL image with schema init
|
|-- catalog/                    # Catalog microservice
|   |-- cmd/catalog/main.go     # Entrypoint
|   |-- pb/                     # Generated protobuf + gRPC Go files
|   |   |-- catalog.pb.go
|   |   |-- catalog_grpc.pb.go
|   |-- catalog.proto           # Protobuf service definition
|   |-- client.go               # gRPC client wrapper
|   |-- server.go               # gRPC server implementation
|   |-- service.go              # Business logic layer (Service interface)
|   |-- repository.go           # Data access layer (Elasticsearch)
|   |-- app.dockerfile          # Multi-stage build for the service binary
|
|-- order/                      # Order microservice
|   |-- cmd/order/main.go       # Entrypoint
|   |-- pb/                     # Generated protobuf + gRPC Go files
|   |   |-- order.pb.go
|   |   |-- order_grpc.pb.go
|   |-- order.proto             # Protobuf service definition
|   |-- client.go               # gRPC client wrapper
|   |-- server.go               # gRPC server implementation
|   |-- service.go              # Business logic layer (Service interface)
|   |-- repository.go           # Data access layer (PostgreSQL)
|   |-- up.sql                  # Database migration / init schema
|   |-- app.dockerfile          # Multi-stage build for the service binary
|   |-- db.dockerfile           # PostgreSQL image with schema init
|
|-- graphql/                    # GraphQL API gateway
|   |-- main.go                 # HTTP server entrypoint
|   |-- graph.go                # Server struct, resolver wiring, executable schema
|   |-- schema.graphql          # GraphQL schema (types, queries, mutations)
|   |-- gqlgen.yaml             # gqlgen configuration
|   |-- models.go               # Custom model overrides (Account)
|   |-- models_gen.go           # Auto-generated models from schema
|   |-- generated.go            # Auto-generated resolver interfaces + execution engine
|   |-- mutation_resolver.go    # Mutation resolver implementations
|   |-- query_resolver.go       # Query resolver implementations
|   |-- account_resolver.go     # Field-level resolver for Account.orders
|   |-- app.dockerfile          # Multi-stage build for the gateway binary
|
|-- vendor/                     # Vendored Go dependencies
|-- docker-compose.yaml         # Full stack orchestration
|-- go.mod                      # Go module definition
|-- go.sum                      # Dependency checksums
|-- .gitignore
```

---

## Services

### Account Service

Manages user accounts. Stores data in PostgreSQL.

**gRPC methods:**

| RPC              | Description                                    |
|------------------|------------------------------------------------|
| `PostAccount`    | Creates a new account with a generated KSUID   |
| `GetAccount`     | Retrieves a single account by ID               |
| `GetAccounts`    | Lists accounts with pagination (skip/take, max 100) |

**Database schema** (`account/up.sql`):

```sql
CREATE TABLE IF NOT EXISTS accounts (
    id CHAR(27) PRIMARY KEY,
    name VARCHAR(24) NOT NULL
);
```

**Internal layers:**
- `service.go` -- Defines the `Service` interface and the `Account` struct. Generates KSUIDs for new accounts.
- `repository.go` -- Defines the `Repository` interface. Implements it with PostgreSQL using `database/sql` and `lib/pq`.
- `server.go` -- Implements the gRPC server. Registers the service with gRPC reflection enabled.
- `client.go` -- Provides a gRPC client wrapper used by other services and the GraphQL gateway.

---

### Catalog Service

Manages product catalog. Stores data in Elasticsearch.

**gRPC methods:**

| RPC              | Description                                             |
|------------------|---------------------------------------------------------|
| `PostProduct`    | Creates a new product with name, description, and price |
| `GetProduct`     | Retrieves a single product by ID                        |
| `GetProducts`    | Lists/searches products with multiple strategies        |

**GetProducts behavior** (determined by request fields):
- If `query` is provided: performs a **multi-match search** across `name` and `description` fields in Elasticsearch.
- If `ids` are provided: fetches products by a list of IDs using Elasticsearch **multi-get**.
- Otherwise: returns all products with pagination (skip/take, max 100).

**Internal layers:**
- `service.go` -- Defines `Service` interface, `Product` struct. Generates KSUIDs for new products.
- `repository.go` -- Implements `Repository` using the `olivere/elastic/v7` client. Indexes products into a `catalog` index.
- `server.go` -- gRPC server with reflection.
- `client.go` -- gRPC client wrapper.

---

### Order Service

Manages orders. Stores data in PostgreSQL. Depends on both Account and Catalog services via gRPC.

**gRPC methods:**

| RPC                    | Description                                                    |
|------------------------|----------------------------------------------------------------|
| `PostOrder`            | Creates a new order for an account with a list of products     |
| `GetOrdersForAccount`  | Retrieves all orders for a given account ID                    |

**PostOrder flow:**
1. Validates the account exists by calling `AccountService.GetAccount`.
2. Fetches product details by calling `CatalogService.GetProducts`.
3. Matches requested product IDs and quantities.
4. Calculates total price (`price * quantity` for each product).
5. Persists the order and its line items in a database transaction using PostgreSQL `COPY` protocol for bulk inserts.

**GetOrdersForAccount flow:**
1. Queries orders and order products from the database using a JOIN.
2. Collects unique product IDs across all orders.
3. Fetches full product details from the Catalog service.
4. Merges product names, descriptions, and prices into the response.

**Database schema** (`order/up.sql`):

```sql
CREATE TABLE IF NOT EXISTS orders(
    id CHAR(27) PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    account_id CHAR(27) NOT NULL,
    total_price MONEY NOT NULL
);

CREATE TABLE IF NOT EXISTS order_products(
    order_id CHAR(27) REFERENCES orders (id) ON DELETE CASCADE,
    product_id CHAR(27),
    quantity INT NOT NULL,
    PRIMARY KEY (product_id, order_id)
);
```

**Internal layers:**
- `service.go` -- Defines `Service` interface, `Order` and `OrderedProduct` structs. Calculates total price on creation.
- `repository.go` -- PostgreSQL repository using transactions and `pq.CopyIn` for bulk order product inserts.
- `server.go` -- gRPC server that instantiates Account and Catalog gRPC clients internally.
- `client.go` -- gRPC client wrapper. Handles time serialization for `CreatedAt`.

---

### GraphQL Gateway

An HTTP server that exposes a unified GraphQL API. It connects to all three backend services via their gRPC clients and translates GraphQL operations into gRPC calls.

**Key files:**
- `main.go` -- Starts an HTTP server on port 8080 (mapped to 8000 externally). Registers the `/graphql` endpoint and a `/playground` endpoint for the GraphQL interactive explorer.
- `graph.go` -- Defines the `Server` struct holding all three gRPC clients. Provides resolver factory methods and builds the `ExecutableSchema`.
- `schema.graphql` -- The full GraphQL schema defining all types, inputs, queries, and mutations.
- `gqlgen.yaml` -- Configuration for gqlgen code generation. Maps the `Account` type to a custom Go model and marks the `orders` field as resolver-based (lazy-loaded).
- `models.go` -- Custom `Account` model with an `Orders` field.
- `mutation_resolver.go` -- Implements `CreateAccount`, `CreateProduct`, and `CreateOrder` mutations. Each uses a 3-second context timeout.
- `query_resolver.go` -- Implements `Accounts` and `Products` queries. Supports fetching by ID or listing with pagination/search.
- `account_resolver.go` -- Field-level resolver for `Account.orders`. When an account is queried, its orders are resolved lazily by calling the Order service.

---

## How to Run

### Using Docker Compose (recommended)

1. Clone the repository:

```bash
git clone https://github.com/dhairyaPandey27/go-grpc-graphql-microservices.git
cd go-grpc-graphql-microservices
```

2. Vendor the dependencies (if the `vendor/` directory is not present):

```bash
go mod vendor
```

3. Build and start all services:

```bash
docker-compose up -d --build
```

This starts seven containers:
- `account` -- Account gRPC service
- `catalog` -- Catalog gRPC service
- `order` -- Order gRPC service
- `graphql` -- GraphQL API gateway
- `account_db` -- PostgreSQL for accounts
- `catalog_db` -- Elasticsearch for products
- `order_db` -- PostgreSQL for orders

4. Access the GraphQL Playground:

Open [http://localhost:8000/playground](http://localhost:8000/playground) in your browser.

5. Send GraphQL requests to:

```
POST http://localhost:8000/graphql
```

6. Stop all services:

```bash
docker-compose down
```

7. Stop and remove all data volumes:

```bash
docker-compose down -v
```

---

## Code Generation

### Generating gRPC / Protobuf Files

Each service has a `.proto` file that defines its gRPC service and messages. The generated Go code lives in the `pb/` subdirectory of each service.

**Generating code for each service:**

Run these commands from the project root directory.

**Account service:**

```bash
protoc --go_out=account/pb --go_opt=paths=source_relative --go-grpc_out=account/pb --go-grpc_opt=paths=source_relative account/account.proto
```

This generates two files in `account/pb/`:
- `account.pb.go` -- Message types (`Account`, `PostAcccountRequest`, `PostAcccountResponse`, `GetAccountRequest`, `GetAccountResponse`, `GetAccountsRequest`, `GetAccountsResponse`)
- `account_grpc.pb.go` -- gRPC service client and server interfaces (`AccountServiceClient`, `AccountServiceServer`)

**Catalog service:**

```bash
protoc --go_out=catalog/pb --go_opt=paths=source_relative --go-grpc_out=catalog/pb --go-grpc_opt=paths=source_relative catalog/catalog.proto
```

This generates two files in `catalog/pb/`:
- `catalog.pb.go` -- Message types (`Product`, `PostProductRequest`, `PostProductResponse`, `GetProductRequest`, `GetProductResponse`, `GetProductsRequest`, `GetProductsResponse`)
- `catalog_grpc.pb.go` -- gRPC service client and server interfaces (`CatalogServiceClient`, `CatalogServiceServer`)

**Order service:**

```bash
protoc --go_out=order/pb --go_opt=paths=source_relative --go-grpc_out=order/pb --go-grpc_opt=paths=source_relative order/order.proto
```

This generates two files in `order/pb/`:
- `order.pb.go` -- Message types (`Order`, `Order_OrderProduct`, `PostOrderRequest`, `PostOrderRequest_OrderProduct`, `PostOrderResponse`, `GetOrderRequest`, `GetOrderResponse`, `GetOrdersForAccountRequest`, `GetOrdersForAccountResponse`)
- `order_grpc.pb.go` -- gRPC service client and server interfaces (`OrderServiceClient`, `OrderServiceServer`)


### Generating GraphQL Files

The GraphQL gateway uses [gqlgen](https://github.com/99designs/gqlgen) for code generation. The configuration is in `graphql/gqlgen.yaml` and the schema is defined in `graphql/schema.graphql`.

**What gets generated:**
- `graphql/generated.go` -- The main execution engine, resolver interfaces (`MutationResolver`, `QueryResolver`, `AccountResolver`), and all the wiring code.
- `graphql/models_gen.go` -- Go structs for the GraphQL types that are not manually defined (everything except `Account`, which is defined in `models.go`).

**To regenerate:**

From the `graphql/` directory:

```bash
cd graphql
go run github.com/99designs/gqlgen generate
```

**Configuration details** (`gqlgen.yaml`):

```yaml
schema: schema.graphql

models:
  Account:
    model: github.com/dhairyaPandey27/go-grpc-graphql-microservices/graphql.Account
    fields:
      orders:
        resolver: true
```

- The `Account` model is mapped to a custom struct in `models.go` instead of using the auto-generated one. This allows the `Orders` field to exist on the struct.
- The `orders` field on `Account` is marked with `resolver: true`, which means gqlgen generates a separate resolver method for it instead of resolving it directly from the struct. This enables lazy loading of orders -- they are only fetched when the client requests the `orders` field on an account.

**When to regenerate:**
- After modifying `schema.graphql` (adding/changing types, queries, mutations).
- After modifying `gqlgen.yaml` (changing model mappings or resolver settings).
- You do NOT need to regenerate after changing resolver implementations (`mutation_resolver.go`, `query_resolver.go`, `account_resolver.go`).

---

## GraphQL API Reference

### Mutations

**Create Account**

```graphql
mutation {
  createAccount(account: { name: "John Doe" }) {
    id
    name
  }
}
```

**Create Product**

```graphql
mutation {
  createProduct(product: {
    name: "Table"
    description: "A wooden table made using premium quality wood"
    price: 99.99
  }) {
    id
    name
    description
    price
  }
}
```

**Create Order**

```graphql
mutation {
  createOrder(order: {
    accountId: "<account-id>"
    products: [
      { id: "<product-id>", quantity: 2 }
    ]
  }) {
    id
    createdAt
    totalPrice
    products {
      id
      name
      description
      price
      quantity
    }
  }
}
```

### Queries

**Get all accounts (with pagination)**

```graphql
query {
  accounts(pagination: { skip: 0, take: 10 }) {
    id
    name
    orders {
      id
      createdAt
      totalPrice
      products {
        id
        name
        quantity
        price
      }
    }
  }
}
```

**Get account by ID**

```graphql
query {
  accounts(id: "<account-id>") {
    id
    name
    orders {
      id
      totalPrice
    }
  }
}
```

**Get all products (with pagination)**

```graphql
query {
  products(pagiation: { skip: 0, take: 10 }) {
    id
    name
    description
    price
  }
}
```

**Search products**

```graphql
query {
  products(query: "laptop") {
    id
    name
    description
    price
  }
}
```

**Get product by ID**

```graphql
query {
  products(id: "<product-id>") {
    id
    name
    description
    price
  }
}
```

---

## Vendor Directory

Dependencies are vendored in the `vendor/` directory. This ensures reproducible builds without network access during Docker builds (all Dockerfiles use `-mod vendor`).

To update vendored dependencies:

```bash
go mod tidy
go mod vendor
```

The `vendor/` directory is listed in `.gitignore` and is not committed to the repository. You must run `go mod vendor` after cloning before building Docker images.

---

## Limitations

- **No authentication or authorization.** There is no user authentication (no passwords, tokens, or sessions on accounts) and no authorization checks on any endpoint. Any client can create, read, or modify any resource.
- **No input validation beyond quantity.** The only validation performed is checking that order product quantities are greater than zero in the GraphQL mutation resolver. Account names, product names, descriptions, and prices are not validated for length, format, or range.
- **No unit or integration tests.** The codebase contains no test files. There are no unit tests for service logic, repository operations, or resolver behavior, and no integration tests for the end-to-end flow.
- **No graceful shutdown.** None of the services handle OS signals (SIGTERM, SIGINT) for graceful shutdown. When a container is stopped, in-flight requests and database connections may be terminated abruptly.
- **No foreign key constraint between orders and accounts.** The `orders` table stores `account_id` as a plain `CHAR(27)` column without a foreign key reference to the `accounts` table (which lives in a different database), so referential integrity is enforced only at the application level.
- **Hardcoded port 8080.** All three gRPC services are hardcoded to listen on port 8080. This works inside Docker (each container has its own network namespace) but makes local development without containers awkward since you cannot run multiple services simultaneously without code changes.
---

## Future Enhancements

- **Authentication and authorization.** Add JWT-based authentication to the GraphQL gateway and propagate identity via gRPC metadata. Implement role-based access control so that, for example, only the owning account can view its orders.
- **Update and delete operations.** Add gRPC methods and GraphQL mutations for updating account details, modifying product information (name, description, price), and canceling orders.
- **Comprehensive testing.** Add unit tests for each service and repository layer using Go's testing package. Add integration tests that spin up the full Docker Compose stack and exercise the GraphQL API end-to-end.
- **Graceful shutdown.** Handle OS signals in each service's main function. Drain in-flight gRPC requests, close database connections cleanly, and shut down the HTTP server with a deadline.
- **TLS and transport security.** Enable TLS on all gRPC connections between services. Serve the GraphQL endpoint over HTTPS. Use proper certificate management for production deployments.
- **Database migration tooling.** Integrate a migration tool like golang-migrate or goose to manage schema changes over time instead of relying solely on init scripts.
- **API gateway features.** Add rate limiting, request size limits, query depth/complexity limits for GraphQL, and CORS configuration to the gateway.
- **Pagination improvements.** Implement cursor-based pagination instead of offset-based skip/take for more efficient and consistent pagination on large datasets.
- **Event-driven communication.** Introduce a message broker (e.g., NATS, Kafka, RabbitMQ) for asynchronous event-driven workflows such as order confirmation notifications, inventory updates, or audit logging.
- **Caching layer.** Add Redis or an in-memory cache in front of frequently read data (e.g., product catalog lookups) to reduce database load and improve response times.
- **Kubernetes readiness.** Add Kubernetes manifests (Deployments, Services, ConfigMaps) or Helm charts to support deploying the system to a Kubernetes cluster beyond Docker Compose.
