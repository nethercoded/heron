create table evidence (
    id          integer primary key,
    monitor_id  integer not null references monitors (id) on delete cascade,
    incident_id integer references incidents (id) on delete cascade,
    at          integer not null,
    report      text    not null,
    image       blob
);

create index evidence_incident on evidence (incident_id, at);
