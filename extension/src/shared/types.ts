export interface Frame {
  type: string;
  payload: any;
}

export interface Update {
  seq: number;
  type: string;
  id: string;
  payload: any;
}

export interface ConnectedPayload {
  session_id: string;
  server_time: string;
  max_seq: number;
}

export interface PageQueryPayload {
  request_id: string;
  include_screenshot: boolean;
}

export interface ActionExecutePayload {
  request_id: string;
  action: string;
  selector: string;
  value?: string;
}
