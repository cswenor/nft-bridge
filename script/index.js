import algosdk, {
  mnemonicToSecretKey,
  signTransaction,
  makePaymentTxnWithSuggestedParamsFromObject,
  waitForConfirmation,
} from "algosdk";
import dotenv from "dotenv";
import figlet from "figlet";
import { algoWalkAssetIds, alienTourismAssetIds } from "./wl.js";
import { signSendAndConfirm } from "./utils.js";

dotenv.config();

console.log(figlet.textSync("NFT BRIDGE", { horizontalLayout: "full" }));

const { ADDR, MN, MN2, MN3, NODE, BRIDGE_ADDR } = process.env;

const bridgeAddress = BRIDGE_ADDR || "";

const addrTo = ADDR;

const mnemonic = MN || "";

const { addr: address, sk } = mnemonicToSecretKey(mnemonic);
const { addr: address2, sk: sk2 } = mnemonicToSecretKey(MN2 || "");
const { addr: address3, sk: sk3 } = mnemonicToSecretKey(MN3 || "");

const [node] = NODE.split(":");

console.log("address", address);
console.log("address2", address2);
console.log("address3", address3);
console.log("bridgeAddress", bridgeAddress);

const [algodClient, indexerClient] = getAlgorandClients(node);

const makeHelloTxn = async () => {
  return makePaymentTxnWithSuggestedParamsFromObject({
    from: address,
    to: bridgeAddress,
    amount: 0,
    note: new Uint8Array(Buffer.from("Hello world")),
    suggestedParams: await algodClient.getTransactionParams().do(),
  });
};

const makeBridgeRequestTxn = async (assetId, addrTo) => {
  const transactionFee = 0.101;
  const serviceFee = 0.099;
  const totalFee = transactionFee + serviceFee;
  const req = {
    assetId,
    to: addrTo,
    amount: 1,
  };
  return makePaymentTxnWithSuggestedParamsFromObject({
    from: address,
    to: bridgeAddress,
    amount: Math.floor(totalFee * 1e6),
    note: new Uint8Array(Buffer.from(JSON.stringify(req))),
    suggestedParams: await algodClient.getTransactionParams().do(),
  });
};

const main = async () => {
  console.log("Do the thing!");
  // say hello world
  await signSendAndConfirm([await makeHelloTxn()], sk);
  // make request to bridge random asset
  await signSendAndConfirm(
    [
      await makeBridgeRequestTxn(
        Math.floor(Math.random() * 10_000_000_000),
        addrTo
      ),
    ],
    sk
  );
  // make request to bridge nfts from collections
  for (const assetIds of [algoWalkAssetIds, alienTourismAssetIds]) {
    await signSendAndConfirm(
      [
        await makeBridgeRequestTxn(
          assetIds[Math.floor(assetIds.length * Math.random())],
          addrTo
        ),
      ],
      sk
    );
  }
};

main();
