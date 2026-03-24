-- Script de inicialização dos bancos de dados Evolution GO

-- Criar database para autenticação
CREATE DATABASE evogo_auth;

-- Criar database para dados de usuários
CREATE DATABASE evogo_users;

-- Mensagem de confirmação
SELECT 'Databases evogo_auth e evogo_users criados com sucesso!' as message;
