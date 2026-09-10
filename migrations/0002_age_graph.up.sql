LOAD 'age';
SET search_path = ag_catalog, "$user", public;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM ag_graph WHERE name = 'product_graph') THEN
        PERFORM create_graph('product_graph');
    END IF;
END
$$;
