--
-- PostgreSQL database dump
--

SET statement_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET check_function_bodies = false;
SET client_min_messages = warning;

--
-- Name: plpgsql; Type: EXTENSION; Schema: -; Owner: 
--

CREATE EXTENSION IF NOT EXISTS plpgsql WITH SCHEMA pg_catalog;


--
-- Name: EXTENSION plpgsql; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION plpgsql IS 'PL/pgSQL procedural language';


SET search_path = public, pg_catalog;

SET default_tablespace = '';

SET default_with_oids = false;

--
-- Name: batch; Type: TABLE; Schema: public; Owner: thompa; Tablespace: 
--

CREATE TABLE batch (
    batch_id uuid NOT NULL,
    user_id integer NOT NULL,
    comment text NOT NULL,
    ctime timestamp with time zone NOT NULL
);


ALTER TABLE public.batch OWNER TO thompa;

--
-- Name: certs; Type: TABLE; Schema: public; Owner: qpov; Tablespace: 
--

CREATE TABLE certs (
    cn character varying(128) NOT NULL,
    user_id integer NOT NULL
);


ALTER TABLE public.certs OWNER TO qpov;

--
-- Name: leases; Type: TABLE; Schema: public; Owner: qpov; Tablespace: 
--

CREATE TABLE leases (
    lease_id uuid NOT NULL,
    done boolean NOT NULL,
    order_id uuid NOT NULL,
    user_id integer,
    created timestamp with time zone NOT NULL,
    updated timestamp with time zone NOT NULL,
    expires timestamp with time zone NOT NULL,
    failed boolean DEFAULT false NOT NULL
);


ALTER TABLE public.leases OWNER TO qpov;

--
-- Name: orders; Type: TABLE; Schema: public; Owner: qpov; Tablespace: 
--

CREATE TABLE orders (
    order_id uuid NOT NULL,
    owner integer NOT NULL,
    definition text NOT NULL,
    batch_id uuid,
    created timestamp with time zone NOT NULL
);


ALTER TABLE public.orders OWNER TO qpov;

--
-- Name: users; Type: TABLE; Schema: public; Owner: qpov; Tablespace: 
--

CREATE TABLE users (
    user_id integer NOT NULL,
    comment text,
    adding boolean DEFAULT false NOT NULL
);


ALTER TABLE public.users OWNER TO qpov;

--
-- Name: users_user_id_seq; Type: SEQUENCE; Schema: public; Owner: qpov
--

CREATE SEQUENCE users_user_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.users_user_id_seq OWNER TO qpov;

--
-- Name: users_user_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: qpov
--

ALTER SEQUENCE users_user_id_seq OWNED BY users.user_id;


--
-- Name: user_id; Type: DEFAULT; Schema: public; Owner: qpov
--

ALTER TABLE ONLY users ALTER COLUMN user_id SET DEFAULT nextval('users_user_id_seq'::regclass);


--
-- Name: batch_pkey; Type: CONSTRAINT; Schema: public; Owner: thompa; Tablespace: 
--

ALTER TABLE ONLY batch
    ADD CONSTRAINT batch_pkey PRIMARY KEY (batch_id);


--
-- Name: certs_pkey; Type: CONSTRAINT; Schema: public; Owner: qpov; Tablespace: 
--

ALTER TABLE ONLY certs
    ADD CONSTRAINT certs_pkey PRIMARY KEY (cn);


--
-- Name: leases_pkey; Type: CONSTRAINT; Schema: public; Owner: qpov; Tablespace: 
--

ALTER TABLE ONLY leases
    ADD CONSTRAINT leases_pkey PRIMARY KEY (lease_id);


--
-- Name: orders_pkey; Type: CONSTRAINT; Schema: public; Owner: qpov; Tablespace: 
--

ALTER TABLE ONLY orders
    ADD CONSTRAINT orders_pkey PRIMARY KEY (order_id);


--
-- Name: users_pkey; Type: CONSTRAINT; Schema: public; Owner: qpov; Tablespace: 
--

ALTER TABLE ONLY users
    ADD CONSTRAINT users_pkey PRIMARY KEY (user_id);


--
-- Name: batch_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: thompa
--

ALTER TABLE ONLY batch
    ADD CONSTRAINT batch_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(user_id);


--
-- Name: certs_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qpov
--

ALTER TABLE ONLY certs
    ADD CONSTRAINT certs_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(user_id);


--
-- Name: leases_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qpov
--

ALTER TABLE ONLY leases
    ADD CONSTRAINT leases_order_id_fkey FOREIGN KEY (order_id) REFERENCES orders(order_id);


--
-- Name: leases_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qpov
--

ALTER TABLE ONLY leases
    ADD CONSTRAINT leases_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(user_id);


--
-- Name: orders_batch_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qpov
--

ALTER TABLE ONLY orders
    ADD CONSTRAINT orders_batch_id_fkey FOREIGN KEY (batch_id) REFERENCES batch(batch_id);


--
-- Name: orders_owner_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qpov
--

ALTER TABLE ONLY orders
    ADD CONSTRAINT orders_owner_fkey FOREIGN KEY (owner) REFERENCES users(user_id);


--
-- Name: public; Type: ACL; Schema: -; Owner: postgres
--

REVOKE ALL ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON SCHEMA public FROM postgres;
GRANT ALL ON SCHEMA public TO postgres;
GRANT ALL ON SCHEMA public TO PUBLIC;


--
-- PostgreSQL database dump complete
--

