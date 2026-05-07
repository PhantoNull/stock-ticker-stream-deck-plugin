import streamDeck from "@elgato/streamdeck";

import { StocksAction } from "./actions/stocks";
streamDeck.logger.setLevel("info");
streamDeck.actions.registerAction(new StocksAction());
streamDeck.connect();
