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

create index idx_check_results_target_time
on check_results (target_name, checked_at desc);

create index idx_check_results_time
on check_results (checked_at desc);