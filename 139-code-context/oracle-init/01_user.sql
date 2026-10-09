-- Runs once when the container is first created (as SYSDBA).
-- HNSW indexes live in the vector memory pool, which must be sized before they can be built.
ALTER SYSTEM SET vector_memory_size = 512M SCOPE = SPFILE;

ALTER SESSION SET CONTAINER = FREEPDB1;
CREATE USER ccctx IDENTIFIED BY ccctx DEFAULT TABLESPACE users QUOTA UNLIMITED ON users;
GRANT DB_DEVELOPER_ROLE, CREATE MINING MODEL, CREATE ANY DIRECTORY TO ccctx;
GRANT EXECUTE ON DBMS_VECTOR TO ccctx;
GRANT EXECUTE ON DBMS_HYBRID_VECTOR TO ccctx;
-- Restart the container once after first boot so vector_memory_size takes effect.
