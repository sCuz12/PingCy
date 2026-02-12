create table incidents (
  id bigserial primary key,
  target_name text not null,
  probe text not null default 'primary',

  started_at timestamptz not null,
  ended_at timestamptz,

  start_status text not null,         -- DOWN / TIMEOUT / BLOCKED
  start_status_code integer,
  start_error text,

  end_status text,                    -- usually UP
  end_status_code integer,
  end_error text,

  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

-- Query latest incidents per target
create index idx_incidents_target_started
on incidents (target_name, started_at desc);

-- Query active incidents fast
create index idx_incidents_active
on incidents (ended_at)
where ended_at is null;

-- Enforce only ONE active incident per (target_name, probe)
create unique index uq_incidents_one_active
on incidents (target_name, probe)
where ended_at is null;