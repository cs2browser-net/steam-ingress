import mysql from 'mysql2/promise';

let connection: mysql.Pool | null = null;

export async function CreateDatabase() {
    if (connection != null) return;
    connection = mysql.createPool(process.env.MYSQL_URL!);
}

export async function GetConnection() {
    if (connection == null) await CreateDatabase();
    return connection;
}

export async function DoQuery(query: string, params: any[] = []): Promise<any[]> {
    if (connection == null) await CreateDatabase();

    const [rows, err] = await connection!.query<any[]>(query, params);
    return rows;
}