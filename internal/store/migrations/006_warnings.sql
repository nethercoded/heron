create table warnings (
    monitor_id  integer not null references monitors (id) on delete cascade,
    kind        text    not null,
    message     text    not null,
    since       integer not null,
    notified_at integer not null default 0,
    primary key (monitor_id, kind)
) without rowid;

create table samples (
    monitor_id integer not null references monitors (id) on delete cascade,
    metric     text    not null,
    at         integer not null,
    value      real    not null,
    total      real    not null default 0,
    primary key (monitor_id, metric, at)
) without rowid;
