-- current_week era un contatore che nessuno incrementava: restava a 1 per
-- sempre, e il calendario in GetProgram ci filtrava sopra. Ora la settimana si
-- calcola da started_at (UserProgram.WeekAt), quindi la colonna non serve più.
--
-- Si toglie invece di lasciarla lì proprio perché una colonna che sembra
-- autorevole e non lo è è il motivo per cui il bug è passato inosservato.
-- Nessun dato da salvare: valeva 1 su ogni riga.
ALTER TABLE user_programs DROP COLUMN IF EXISTS current_week;
