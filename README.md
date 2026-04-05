# comment-service

Сервис с GraphQL API для постов и иерархических комментариев.

## Быстрый запуск

### Локально с PostgreSQL

```bash
make docker-up
make migrate-up
make generate
go run ./cmd/comment-service
```

По умолчанию сервис поднимается на `:8080`.
- `http://localhost:8080/playground`
### Через Docker Compose

```bash
docker compose up --build
```

## Ключевые переменные окружения

- `COMMENT_SERVICE_ENV=local|dev|prod`
- `COMMENT_SERVICE_STORAGE_BACKEND=memory|postgres`
- `COMMENT_SERVICE_HTTP_ADDR=:8080`
- `COMMENT_SERVICE_POSTGRES_DSN=postgres://comment:comment@localhost:5433/comment_service?sslmode=disable`
- `COMMENT_SERVICE_GRAPHQL_ALLOWED_ORIGINS=http://localhost:3000`

Для `setCommentsEnabled` сервис ожидает acting user в HTTP заголовке `X-Actor-ID`.

## Примеры GraphQL

### Список постов

```graphql
query ListPosts {
  posts(first: 10, after: null) {
    totalCount
    edges {
      cursor
      node {
        id
        authorId
        title
        commentsEnabled
        createdAt
      }
    }
    pageInfo {
      hasNextPage
      endCursor
    }
  }
}
```

### Создание комментария

```graphql
mutation CreateComment($input: CreateCommentInput!) {
  createComment(input: $input) {
    id
    postId
    parentId
    authorId
    body
    createdAt
  }
}
```

Пример variables:

```json
{
  "input": {
    "postId": "11111111-1111-1111-1111-111111111111",
    "parentId": null,
    "authorId": "22222222-2222-2222-2222-222222222222",
    "body": "Первый комментарий"
  }
}
```

### Включение или выключение комментариев

```graphql
mutation ToggleComments($postId: UUID!, $enabled: Boolean!) {
  setCommentsEnabled(postId: $postId, enabled: $enabled) {
    id
    commentsEnabled
  }
}
```

HTTP header:

```text
X-Actor-ID: 22222222-2222-2222-2222-222222222222
```

### Подписка на новые комментарии

```graphql
subscription WatchComments($postId: UUID!) {
  commentAdded(postId: $postId) {
    id
    postId
    parentId
    authorId
    body
    createdAt
  }
}
```
