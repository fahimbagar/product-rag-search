LOAD 'age';
SET search_path = ag_catalog, "$user", public;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM ag_graph WHERE name = 'product_graph') THEN
        PERFORM drop_graph('product_graph', true);
    END IF;
END
$$;
