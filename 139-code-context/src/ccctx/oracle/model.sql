-- Load the all-MiniLM-L12-v2 sentence-transformer into the database.
--
-- Oracle publishes an "augmented" ONNX export of the model (tokenizer and
-- pooling baked in) that DBMS_VECTOR can load directly: all_MiniLM_L12_v2_augmented.zip,
-- linked from the AI Vector Search User's Guide ("Import Pretrained Models in ONNX Format").
-- Unzip it into the directory mounted at /opt/oracle/models (docker-compose.yml).
-- As SYSDBA: GRANT CREATE MINING MODEL, CREATE ANY DIRECTORY TO ccctx;

CREATE OR REPLACE DIRECTORY ccctx_models AS '/opt/oracle/models';

BEGIN
    DBMS_VECTOR.DROP_ONNX_MODEL(model_name => 'ALL_MINILM_L12_V2', force => TRUE);
    DBMS_VECTOR.LOAD_ONNX_MODEL(
        directory  => 'CCCTX_MODELS',
        file_name  => 'all_MiniLM_L12_v2.onnx',
        model_name => 'ALL_MINILM_L12_V2');
END;
/

-- Smoke test: should print a 384-dimensional FLOAT32 vector.
SELECT VECTOR_DIMENSION_COUNT(VECTOR_EMBEDDING(ALL_MINILM_L12_V2 USING 'hello world' AS data)) AS dims
FROM dual;
