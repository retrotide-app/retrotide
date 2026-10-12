\getenv app_password RETROTIDE_DB_PASSWORD
\getenv monitor_password RETROTIDE_MONITOR_PASSWORD

CREATE ROLE retrotide LOGIN PASSWORD :'app_password';
CREATE DATABASE retrotide OWNER retrotide;

CREATE ROLE retrotide_monitor LOGIN PASSWORD :'monitor_password';
GRANT pg_monitor TO retrotide_monitor;
