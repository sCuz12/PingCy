create table check_results (
    id bigserial primary key,
    target_name text not null,
    checked_at timestamptz not null default now(),
    status text not null, -- UP / DOWN / TIMEOUT / BLOCKED
    status_code integer,
    latency_ms integer,
    error text,
    probe text not null default 'primary'
);