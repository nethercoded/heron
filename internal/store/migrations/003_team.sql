alter table users add column role text not null default 'admin';
alter table users add column totp_secret text not null default '';
alter table users add column totp_step integer not null default 0;
alter table users add column handle blob;
alter table users add column last_seen_at integer not null default 0;

update users set role = 'owner' where id = (select min(id) from users);
update users set handle = randomblob(32);

create unique index users_handle on users (handle);

create table invites (
    id         integer primary key,
    token_hash blob    not null unique,
    role       text    not null,
    created_by integer references users (id) on delete set null,
    created_at integer not null,
    expires_at integer not null
);

create table passkeys (
    id           blob    primary key,
    user_id      integer not null references users (id) on delete cascade,
    name         text    not null,
    credential   blob    not null,
    created_at   integer not null,
    last_used_at integer not null default 0
) without rowid;

create index passkeys_user on passkeys (user_id);

create table audit_log (
    id       integer primary key,
    at       integer not null,
    user_id  integer references users (id) on delete set null,
    username text    not null,
    action   text    not null,
    target   text    not null default '',
    ip       text    not null default ''
);

create index audit_log_at on audit_log (at);
