create table posts (
    id uuid primary key,
    author_id uuid not null,
    title text not null,
    content text not null,
    comments_enabled boolean not null default true,
    created_at timestamptz not null
);

create table comments (
    id uuid primary key,
    post_id uuid not null references posts(id) on delete cascade,
    parent_id uuid null references comments(id) on delete cascade,
    author_id uuid not null,
    body text not null,
    created_at timestamptz not null,
    check (char_length(body) <= 2000)
);

create index idx_posts_created_at_id_desc
    on posts (created_at desc, id desc);

create index idx_comments_post_parent_created_at_id_desc
    on comments (post_id, parent_id, created_at desc, id desc);

create index idx_comments_parent_created_at_id_desc
    on comments (parent_id, created_at desc, id desc);
